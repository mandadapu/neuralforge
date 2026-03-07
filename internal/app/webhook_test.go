package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// errReader is an io.Reader that always returns an error.
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated read error")
}

// errResponseWriter is an http.ResponseWriter whose Write always fails.
type errResponseWriter struct {
	header     http.Header
	statusCode int
}

func newErrResponseWriter() *errResponseWriter {
	return &errResponseWriter{header: make(http.Header)}
}

func (w *errResponseWriter) Header() http.Header         { return w.header }
func (w *errResponseWriter) WriteHeader(statusCode int)   { w.statusCode = statusCode }
func (w *errResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("simulated write error")
}

func TestGhAppWebhookHandler_Success(t *testing.T) {
	app := &App{}
	handler := app.ghAppWebhookHandler()

	body := `{"action":"push"}`
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"ok":true}`, rr.Body.String())
}

func TestGhAppWebhookHandler_ReadError(t *testing.T) {
	app := &App{}
	handler := app.ghAppWebhookHandler()

	req := httptest.NewRequest("POST", "/webhooks/github", nil)
	req.Body = io.NopCloser(errReader{})
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGhAppWebhookHandler_WriteError(t *testing.T) {
	app := &App{}
	handler := app.ghAppWebhookHandler()

	body := `{"action":"push"}`
	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")

	w := newErrResponseWriter()
	// Should not panic even when Write fails.
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.statusCode)
}
