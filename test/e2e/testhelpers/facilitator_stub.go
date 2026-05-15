// Package testhelpers provides test infrastructure for e2e tests.
package testhelpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
)

// FacilitatorStub is a configurable httptest.Server that mimics an x402 facilitator.
// It handles /supported, /verify, and /settle endpoints.
// It tracks nonces to reject replays after successful settlement.
type FacilitatorStub struct {
	Server *httptest.Server

	// VerifyValid controls whether /verify returns isValid=true.
	VerifyValid atomic.Bool

	// SettleSuccess controls whether /settle returns success=true.
	SettleSuccess atomic.Bool

	// VerifyCallCount tracks how many times /verify was called.
	VerifyCallCount atomic.Int64

	// SettleCallCount tracks how many times /settle was called.
	SettleCallCount atomic.Int64

	// settledNonces tracks nonces that have already been settled.
	// After a successful settlement, the same nonce is rejected.
	settledNonces   map[string]struct{}
	settledNoncesMu sync.Mutex
}

// NewFacilitatorStub creates and starts a new facilitator stub server.
// verifyValid and settleSuccess control initial stub behavior.
func NewFacilitatorStub(verifyValid, settleSuccess bool) *FacilitatorStub {
	s := &FacilitatorStub{
		settledNonces: make(map[string]struct{}),
	}
	s.VerifyValid.Store(verifyValid)
	s.SettleSuccess.Store(settleSuccess)

	mux := http.NewServeMux()
	mux.HandleFunc("/supported", s.handleSupported)
	mux.HandleFunc("/verify", s.handleVerify)
	mux.HandleFunc("/settle", s.handleSettle)

	s.Server = httptest.NewServer(mux)
	return s
}

// Close shuts down the stub server.
func (s *FacilitatorStub) Close() {
	s.Server.Close()
}

// URL returns the base URL of the stub server.
func (s *FacilitatorStub) URL() string {
	return s.Server.URL
}

// handleSupported responds with the set of supported payment kinds.
func (s *FacilitatorStub) handleSupported(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]interface{}{
		"kinds": []map[string]interface{}{
			{
				"x402Version": 2,
				"scheme":      "exact",
				"network":     "eip155:84532",
			},
			{
				"x402Version": 2,
				"scheme":      "exact",
				"network":     "eip155:8453",
			},
		},
		"extensions": []string{},
		"signers":    map[string][]string{},
	}
	writeJSON(w, http.StatusOK, resp)
}

// extractNonce attempts to extract a nonce from the verify request body.
// It reads the payment payload's authorization.nonce field.
func extractNonce(r *http.Request) string {
	var body struct {
		PaymentPayload struct {
			Payload struct {
				Authorization struct {
					Nonce string `json:"nonce"`
				} `json:"authorization"`
			} `json:"payload"`
		} `json:"paymentPayload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ""
	}
	return body.PaymentPayload.Payload.Authorization.Nonce
}

// handleVerify responds with a canned verification result.
// It checks if the nonce has already been settled (replay protection).
func (s *FacilitatorStub) handleVerify(w http.ResponseWriter, r *http.Request) {
	s.VerifyCallCount.Add(1)

	if !s.VerifyValid.Load() {
		// Return an error response to trigger the SDK's error path.
		resp := map[string]interface{}{
			"isValid":        false,
			"invalidReason":  "insufficient_funds",
			"invalidMessage": "stubbed rejection",
		}
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}

	// Check for replay: if this nonce was already settled, reject.
	nonce := extractNonce(r)
	if nonce != "" {
		s.settledNoncesMu.Lock()
		_, alreadySettled := s.settledNonces[nonce]
		s.settledNoncesMu.Unlock()
		if alreadySettled {
			resp := map[string]interface{}{
				"isValid":        false,
				"invalidReason":  "nonce_already_used",
				"invalidMessage": "this nonce has already been settled",
			}
			writeJSON(w, http.StatusBadRequest, resp)
			return
		}
	}

	resp := map[string]interface{}{
		"isValid": true,
		"payer":   "0x0000000000000000000000000000000000000001",
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSettle responds with a canned settlement result.
// On success, marks the nonce as used to prevent replay.
func (s *FacilitatorStub) handleSettle(w http.ResponseWriter, r *http.Request) {
	s.SettleCallCount.Add(1)

	if !s.SettleSuccess.Load() {
		resp := map[string]interface{}{
			"success":     false,
			"errorReason": "settlement_failed",
		}
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}

	// Mark nonce as settled to prevent replay.
	nonce := extractNonce(r)
	if nonce != "" {
		s.settledNoncesMu.Lock()
		s.settledNonces[nonce] = struct{}{}
		s.settledNoncesMu.Unlock()
	}

	resp := map[string]interface{}{
		"success":     true,
		"transaction": "0xdeadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		"network":     "eip155:84532",
		"payer":       "0x0000000000000000000000000000000000000001",
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
