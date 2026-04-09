package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookSignatureValidation(t *testing.T) {
	secret := "test-secret"
	body := `{"action":"labeled"}`

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	handler := NewWebhookHandler(secret, func(eventType string, payload []byte) {})

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookRejectsOversizedPayload(t *testing.T) {
	handler := NewWebhookHandler("secret", func(eventType string, payload []byte) {
		t.Fatal("callback should not be invoked for oversized payload")
	})

	// Create a payload that exceeds the 25MB limit
	oversized := strings.Repeat("x", 25*1024*1024+1)
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(oversized))
	req.Header.Set("X-Hub-Signature-256", "sha256=irrelevant")
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	handler := NewWebhookHandler("secret", func(eventType string, payload []byte) {})

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader("{}"))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// failingResponseWriter simulates a ResponseWriter whose Write() always fails.
type failingResponseWriter struct {
	header http.Header
	code   int
}

func (f *failingResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

func (f *failingResponseWriter) WriteHeader(code int) {
	f.code = code
}

func TestGhAppWebhookHandlerWriteError(t *testing.T) {
	a := &App{}
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader("{}"))
	req.Header.Set("X-GitHub-Event", "push")

	rw := &failingResponseWriter{}
	assert.NotPanics(t, func() {
		a.ghAppWebhookHandler(rw, req)
	})
	assert.Equal(t, http.StatusOK, rw.code)
}
