package main

// storage abstracts persistence so the server can run on local SQLite or
// Cloudflare D1 (via REST API) without changing handlers.
type storage interface {
	seedAdmin(username, password string) error
	verifyAdmin(username, password string) (bool, error)
	createAPIKey(label string) (string, error)
	validAPIKey(raw string) (bool, error)
	createTxn(reference, amount, merchant, qris string) error
	updateTxnStatus(reference, status, provider, providerRef string) (bool, error)
	getTxn(reference string) (*txnRow, error)
	listTxns(limit int) ([]txnRow, error)
}

type txnRow struct {
	ID          int64  `json:"id"`
	Reference   string `json:"reference"`
	Amount      string `json:"amount"`
	Merchant    string `json:"merchant"`
	Status      string `json:"status"`
	Provider    string `json:"provider,omitempty"`
	ProviderRef string `json:"provider_ref,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
