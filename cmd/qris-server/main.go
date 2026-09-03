package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/skip2/go-qrcode"
	"github.com/verssache/qris-dinamis-go/internal/qris"
)

// loadEnvFile loads the nearest .env walking up from the working directory
// so the server finds config regardless of where `go run` is invoked.
func loadEnvFile() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for depth := 0; depth < 6; depth++ {
		path := filepath.Join(dir, ".env")
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			values, readErr := godotenv.Read(path)
			if readErr != nil {
				log.Printf("WARNING: failed to read env file %s: %v", path, readErr)
				return
			}
			for key, value := range values {
				if current, exists := os.LookupEnv(key); !exists || current == "" {
					_ = os.Setenv(key, value)
				}
			}
			log.Printf("loaded env from %s", path)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

type validateRequest struct {
	QRIS string `json:"qris"`
}

type parseRequest struct {
	QRIS string `json:"qris"`
}

type convertRequest struct {
	QRIS   string           `json:"qris"`
	Amount string           `json:"amount"`
	Fee    *qris.ConvertFee `json:"fee,omitempty"`
}

type convertResponse struct {
	Result string    `json:"result"`
	Parsed qris.Data `json:"parsed"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result := qris.Validate(req.QRIS)
	writeJSON(w, http.StatusOK, result)
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	data := qris.Parse(req.QRIS)
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req convertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	validation := qris.Validate(req.QRIS)
	if !validation.Valid {
		writeError(w, http.StatusBadRequest, strings.Join(validation.Errors, "; "))
		return
	}
	parsed := qris.Parse(req.QRIS)
	if parsed.Method == qris.MethodDynamic {
		writeError(w, http.StatusBadRequest, "QRIS is already dynamic")
		return
	}
	amount := strings.TrimSpace(req.Amount)
	if amount == "" {
		writeError(w, http.StatusBadRequest, "amount is required")
		return
	}
	if n, err := strconv.Atoi(amount); err != nil || n <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be a positive integer")
		return
	}
	result := qris.Convert(req.QRIS, qris.ConvertOptions{Amount: amount, Fee: req.Fee})
	writeJSON(w, http.StatusOK, convertResponse{Result: result, Parsed: qris.Parse(result)})
}

func handleQR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	data := r.URL.Query().Get("data")
	if data == "" {
		writeError(w, http.StatusBadRequest, "missing data query param")
		return
	}
	size := 280
	if s := r.URL.Query().Get("size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 2048 {
			size = n
		}
	}
	png, err := qrcode.Encode(data, qrcode.Medium, size)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to generate QR: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func main() {
	loadEnvFile()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	_ = os.MkdirAll(dataDir, 0o755)

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = dataDir + "/qris.db"
	}

	var store storage
	var closeStore func()
	if os.Getenv("STORAGE") == "d1" {
		d1, err := openD1Store(d1Config{
			AccountID:  os.Getenv("D1_ACCOUNT_ID"),
			DatabaseID: os.Getenv("D1_DATABASE_ID"),
			APIToken:   os.Getenv("CLOUDFLARE_API_TOKEN"),
		})
		if err != nil {
			log.Fatalf("failed to open D1 store: %v", err)
		}
		store = d1
		log.Printf("storage: Cloudflare D1")
	} else {
		local, err := openStore(dbDSN)
		if err != nil {
			log.Fatalf("failed to open store: %v", err)
		}
		closeStore = func() { _ = local.db.Close() }
		store = local
		log.Printf("storage: local SQLite (%s)", dbDSN)
	}
	if closeStore != nil {
		defer closeStore()
	}
	backend, ok := store.(migrationBackend)
	if !ok {
		log.Fatal("selected storage does not support migrations")
	}
	if err := runMigrations(findMigrationsDir(), backend); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := os.Getenv("ADMIN_PASS")
	adminEnabled := len(adminPass) >= 8
	if !adminEnabled {
		log.Printf("WARNING: ADMIN_PASS tidak di-set atau < 8 karakter. Login admin dinonaktifkan (HTTP 503). Set ADMIN_PASS di environment.")
	} else if err := store.seedAdmin(adminUser, adminPass); err != nil {
		log.Fatalf("failed to seed admin: %v", err)
	}

	sessions := newSessionStore(0)
	webhooks := newWebhookManager()
	provider := &stubProvider{name: "stub"}

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
		if _, err := os.Stat(webDir + "/index.html"); err != nil {
			webDir = "../web"
			if _, err := os.Stat(webDir + "/index.html"); err != nil {
				webDir = "../../web"
			}
		}
	}

	mux := http.NewServeMux()

	// public converters
	mux.HandleFunc("/api/validate", handleValidate)
	mux.HandleFunc("/api/parse", handleParse)
	mux.HandleFunc("/api/convert", handleConvert)
	mux.HandleFunc("/api/qr", handleQR)
	mux.HandleFunc("/api/batch-zip", handleBatchZIP)
	mux.HandleFunc("/api/batch-csv", handleBatchCSV)
	mux.HandleFunc("/api/pdf", handlePDF)

	// public txn creation (POS-style) + admin list + webhook + status stub
	mux.Handle("/api/transactions", requireAPIKey(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handleTxnCreate(w, r, store) })))
	mux.Handle("/api/transactions/list", requireAdmin(store, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handleTxnList(w, r, store) })))
	mux.HandleFunc("/api/webhook", func(w http.ResponseWriter, r *http.Request) { handleWebhook(w, r, store, webhooks) })
	mux.HandleFunc("/api/status/", func(w http.ResponseWriter, r *http.Request) { handleStatusCheck(w, r, store, webhooks, provider) })

	// admin auth
	mux.HandleFunc("/api/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if !adminEnabled {
			writeError(w, http.StatusServiceUnavailable, "admin not configured: ADMIN_PASS is missing or too short")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		ok, err := store.verifyAdmin(req.Username, req.Password)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		token := randomHex(24)
		sessions.create(token)
		http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.Handle("/api/admin/keys", requireAdmin(store, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !adminEnabled {
			writeError(w, http.StatusServiceUnavailable, "admin not configured: ADMIN_PASS is missing or too short")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req struct {
			Label string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		key, err := store.createAPIKey(req.Label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create key")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_key": key, "note": "store this key now; it will not be shown again"})
	})))

	// SPA static: serve files, fall back to index.html for non-API routes
	var fileServer http.Handler
	if _, err := os.Stat(webDir + "/index.html"); err == nil {
		fileServer = spaFileServer(http.Dir(webDir))
	} else {
		fileServer = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "web/ not found — build the frontend first")
		})
	}
	mux.Handle("/", fileServer)

	log.Printf("QRIS Dinamis server listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
