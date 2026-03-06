package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxWebhookPayloadBytes = 25 * 1024 * 1024 // 25 MB — GitHub's documented limit

type EventCallback func(eventType string, payload []byte)

type WebhookHandler struct {
	secret   string
	callback EventCallback
}

func NewWebhookHandler(secret string, callback EventCallback) *WebhookHandler {
	return &WebhookHandler{secret: secret, callback: callback}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookPayloadBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "read error", http.StatusBadRequest)
		}
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !h.verifySignature(body, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	go h.callback(eventType, body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	return hmac.Equal(sig, mac.Sum(nil))
}
