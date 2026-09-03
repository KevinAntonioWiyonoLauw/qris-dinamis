package main

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---- auth ----

func requireAPIKey(s storage) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				writeError(w, http.StatusUnauthorized, "missing X-API-Key header")
				return
			}
			ok, err := s.validAPIKey(key)
			if err != nil || !ok {
				writeError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			next(w, r)
		}
	}
}

func requireAdmin(s storage, sessions *sessionStore) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value == "" || !sessions.valid(cookie.Value) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next(w, r)
		}
	}
}

// ---- sessions (in-memory, simple) ----

type sessionStore struct {
	mu      sync.Mutex
	tokens  map[string]time.Time
	timeout time.Duration
}

func newSessionStore(timeout time.Duration) *sessionStore {
	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	return &sessionStore{tokens: map[string]time.Time{}, timeout: timeout}
}

func (ss *sessionStore) create(token string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.tokens[token] = time.Now().Add(ss.timeout)
}

func (ss *sessionStore) valid(token string) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	exp, ok := ss.tokens[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(ss.tokens, token)
		return false
	}
	return true
}

// ---- payment provider stub ----
//
// QRIS string alone cannot report payment status. Concrete providers
// (DANA / OVO / GoPay / Midtrans / Xendit) require their own API keys,
// merchant IDs and webhook signatures. These types form the integration
// surface; wire a provider adapter once credentials exist.

type PaymentStatus struct {
	Reference   string `json:"reference"`
	Status      string `json:"status"`
	Provider    string `json:"provider,omitempty"`
	ProviderRef string `json:"provider_ref,omitempty"`
	Message     string `json:"message,omitempty"`
}

type paymentProvider interface {
	Name() string
	CheckStatus(ref string) (*PaymentStatus, error)
}

type stubProvider struct{ name string }

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) CheckStatus(ref string) (*PaymentStatus, error) {
	return &PaymentStatus{Reference: ref, Status: "unknown", Provider: p.name, Message: "provider not wired: add API credentials"}, nil
}

type webhookManager struct {
	mu      sync.Mutex
	events  map[string][]string
	maxKeep int
}

func newWebhookManager() *webhookManager {
	return &webhookManager{events: map[string][]string{}, maxKeep: 20}
}

func (wm *webhookManager) record(reference, status string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	list := wm.events[reference]
	list = append(list, status)
	if len(list) > wm.maxKeep {
		list = list[len(list)-wm.maxKeep:]
	}
	wm.events[reference] = list
}

func (wm *webhookManager) eventsFor(reference string) []string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return append([]string{}, wm.events[reference]...)
}

func handleStatusCheck(w http.ResponseWriter, r *http.Request, s storage, wm *webhookManager, provider paymentProvider) {
	ref := strings.TrimSpace(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
	if ref == "" {
		writeError(w, http.StatusBadRequest, "reference required")
		return
	}
	txn, err := s.getTxn(ref)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load transaction")
		return
	}
	if txn == nil {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	providerStatus, _ := provider.CheckStatus(ref)
	writeJSON(w, http.StatusOK, map[string]any{
		"reference":    txn.Reference,
		"amount":       txn.Amount,
		"merchant":     txn.Merchant,
		"status":       txn.Status,
		"provider":     provider.Name(),
		"provider_ref": txn.ProviderRef,
		"webhook_log":  wm.eventsFor(ref),
		"note":         providerStatus.Message,
	})
}
