package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
	"github.com/verssache/qris-dinamis-go/internal/qris"
)

type batchRequest struct {
	QRIS  string `json:"qris"`
	Items []struct {
		Nominal string `json:"nominal"`
		Code    string `json:"code"`
	} `json:"items"`
}

func handleBatchZIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items required")
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, item := range req.Items {
		png, err := qrcode.Encode(item.Code, qrcode.Medium, 280)
		if err != nil {
			zw.Close()
			writeError(w, http.StatusBadRequest, "failed to generate QR: "+err.Error())
			return
		}
		f, err := zw.Create(fmt.Sprintf("qris-%d-rp%s.png", i+1, item.Nominal))
		if err != nil {
			zw.Close()
			writeError(w, http.StatusInternalServerError, "failed to create zip entry")
			return
		}
		_, _ = f.Write(png)
	}
	if err := zw.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finalize zip")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="batch-qris.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

func handleBatchCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items required")
		return
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"merchant", "nominal", "qris"})
	for _, item := range req.Items {
		_ = cw.Write([]string{req.QRIS, item.Nominal, item.Code})
	}
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv;charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="batch-qris.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

type pdfRequest struct {
	QRIS string `json:"qris"`
}

func handlePDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req pdfRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.QRIS == "" {
		writeError(w, http.StatusBadRequest, "qris required")
		return
	}
	parsed := qris.Parse(req.QRIS)

	png, err := qrcode.Encode(req.QRIS, qrcode.Medium, 512)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to generate QR: "+err.Error())
		return
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 12, "QRIS Dinamis")
	pdf.Ln(20)
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(0, 8, "Merchant: "+parsed.MerchantName)
	pdf.Ln(8)
	pdf.Cell(0, 8, "Kota: "+parsed.MerchantCity)
	pdf.Ln(8)
	if parsed.Amount != "" {
		pdf.Cell(0, 8, "Nominal: Rp "+parsed.Amount)
		pdf.Ln(8)
	}
	pdf.Ln(8)

	reader := bytes.NewReader(png)
	opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	pdf.RegisterImageReader("qr", "png", reader)
	pdf.ImageOptions("qr", 70, 80, 70, 0, false, opt, 0, "")
	if err := pdf.Error(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build pdf: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="qris.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	_ = pdf.Output(w)
}

// ---- transactions API ----

type txnRequest struct {
	QRIS      string `json:"qris"`
	Amount    string `json:"amount"`
	Merchant  string `json:"merchant"`
	Reference string `json:"reference,omitempty"`
}

func handleTxnCreate(w http.ResponseWriter, r *http.Request, s storage) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req txnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.QRIS == "" || req.Amount == "" {
		writeError(w, http.StatusBadRequest, "qris and amount required")
		return
	}
	if n, err := strconv.Atoi(req.Amount); err != nil || n <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be a positive integer")
		return
	}
	ref := req.Reference
	if ref == "" {
		ref = fmt.Sprintf("QRIS-%d", time.Now().UnixNano())
	}
	parsed := qris.Parse(req.QRIS)
	converted := qris.Convert(req.QRIS, qris.ConvertOptions{Amount: req.Amount})
	merchant := req.Merchant
	if merchant == "" {
		merchant = parsed.MerchantName
	}
	if err := s.createTxn(ref, req.Amount, merchant, converted); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create transaction")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"reference": ref,
		"amount":    req.Amount,
		"merchant":  merchant,
		"qris":      converted,
		"status":    "pending",
	})
}

func handleTxnList(w http.ResponseWriter, _ *http.Request, s storage) {
	txns, err := s.listTxns(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txns})
}

type webhookRequest struct {
	Reference   string `json:"reference"`
	Status      string `json:"status"`
	Provider    string `json:"provider"`
	ProviderRef string `json:"provider_ref"`
}

func handleWebhook(w http.ResponseWriter, r *http.Request, s storage, wh *webhookManager) {
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	validStatus := map[string]bool{"success": true, "failed": true, "pending": true, "expired": true}
	if req.Reference == "" || !validStatus[req.Status] {
		writeError(w, http.StatusBadRequest, "reference and valid status required")
		return
	}
	updated, err := s.updateTxnStatus(req.Reference, req.Status, req.Provider, req.ProviderRef)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	wh.record(req.Reference, req.Status)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
