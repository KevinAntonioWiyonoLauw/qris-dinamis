// Package qris implements QRIS (Quick Response Code Indonesian Standard)
// TLV parsing, validation, and static-to-dynamic conversion.
//
// This is a 1:1 port of the TypeScript core from
// https://github.com/KevinAntonioWiyonoLauw (src/core).
package qris

// TLV is a single Tag-Length-Value element from a QRIS payload.
type TLV struct {
	Tag    string `json:"tag"`
	Name   string `json:"name"`
	Length int    `json:"length"`
	Value  string `json:"value"`
	// Children holds nested TLV sub-elements (merchant account info tags 26-51 and tag 62).
	Children []TLV `json:"children,omitempty"`
}

// MerchantAccountInfo describes one payment-provider account block (tags 26-51).
type MerchantAccountInfo struct {
	Tag              string `json:"tag"`
	GloballyUniqueID string `json:"globallyUniqueId"`
	MerchantID       string `json:"merchantId,omitempty"`
	MerchantCriteria string `json:"merchantCriteria,omitempty"`
	Fields           []TLV  `json:"fields"`
}

// Method is the Point of Initiation value: "static" or "dynamic".
type Method string

const (
	MethodStatic  Method = "static"
	MethodDynamic Method = "dynamic"
)

// TipIndicator describes the Tip or Convenience Indicator (tag 55).
type TipIndicator string

const (
	TipPrompt     TipIndicator = "prompt"
	TipFixed      TipIndicator = "fixed"
	TipPercentage TipIndicator = "percentage"
)

// Data is a parsed QRIS string in a human-friendly structure.
type Data struct {
	Version              string                `json:"version"`
	Method               Method                `json:"method"`
	MerchantAccountInfo  []MerchantAccountInfo `json:"merchantAccountInfo"`
	MerchantCategoryCode string                `json:"merchantCategoryCode"`
	Currency             string                `json:"currency"`
	Amount               string                `json:"amount,omitempty"`
	TipIndicator         TipIndicator          `json:"tipIndicator,omitempty"`
	TipFixed             string                `json:"tipFixed,omitempty"`
	TipPercentage        string                `json:"tipPercentage,omitempty"`
	CountryCode          string                `json:"countryCode"`
	MerchantName         string                `json:"merchantName"`
	MerchantCity         string                `json:"merchantCity"`
	PostalCode           string                `json:"postalCode"`
	AdditionalData       []TLV                 `json:"additionalData,omitempty"`
	CRC                  string                `json:"crc"`
	Raw                  []TLV                 `json:"raw"`
}

// FeeType is the service-fee kind: fixed or percentage.
type FeeType string

const (
	FeeFixed      FeeType = "fixed"
	FeePercentage FeeType = "percentage"
)

// ConvertOptions controls static-to-dynamic conversion.
type ConvertOptions struct {
	Amount string      `json:"amount"`
	Fee    *ConvertFee `json:"fee,omitempty"`
}

// ConvertFee is an optional service fee attached to a conversion.
type ConvertFee struct {
	Type  FeeType `json:"type"`
	Value float64 `json:"value"`
}

// ValidationResult reports structural validity of a QRIS string.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}
