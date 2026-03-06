# ghappauth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a self-contained Go module (`github.com/mandadapu/ghappauth`) for GitHub App authentication — JWT signing, installation token exchange/caching, webhook verification, auto-refreshing HTTP transport, and git credential helpers.

**Architecture:** Flat package structure with all exports in the root package. No external JWT library — uses stdlib `crypto/rsa` + `encoding/json`. Token caching is in-memory with `sync.RWMutex`. Tests use `crypto/rand` for fake RSA keys and `net/http/httptest` for mock GitHub API.

**Tech Stack:** Go 1.22+, stdlib crypto, `net/http/httptest` for tests

**Prerequisites:** Create the `ghappauth` repo on GitHub: `gh repo create mandadapu/ghappauth --public --clone`

---

### Task 1: Initialize Go Module + Errors + Config

**Files:**
- Create: `go.mod`
- Create: `ghappauth.go`
- Create: `errors.go`
- Create: `ghappauth_test.go`

**Step 1: Initialize the Go module**

```bash
cd /Users/suryamandadapu/src
gh repo create mandadapu/ghappauth --public --description "Reusable GitHub App authentication for Go" --clone
cd ghappauth
go mod init github.com/mandadapu/ghappauth
```

**Step 2: Write the failing test**

Create `ghappauth_test.go`:

```go
package ghappauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// testKey generates a throwaway RSA key for testing.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// testPEMFile writes a PEM-encoded RSA private key to a temp file.
func testPEMFile(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pem")
	data := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNew_WithPrivateKeyPath(t *testing.T) {
	key := testKey(t)
	path := testPEMFile(t, key)

	app, err := New(Config{
		AppID:          12345,
		PrivateKeyPath: path,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.appID != 12345 {
		t.Errorf("appID = %d, want 12345", app.appID)
	}
}

func TestNew_WithPrivateKeyBytes(t *testing.T) {
	key := testKey(t)
	data := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	app, err := New(Config{
		AppID:      12345,
		PrivateKey: data,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_InvalidKey(t *testing.T) {
	_, err := New(Config{
		AppID:      12345,
		PrivateKey: []byte("not a pem key"),
	})
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestNew_MissingKey(t *testing.T) {
	_, err := New(Config{
		AppID: 12345,
	})
	if err == nil {
		t.Fatal("expected error when no key provided")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test -v -run TestNew`
Expected: FAIL — `New` undefined

**Step 4: Write minimal implementation**

Create `errors.go`:

```go
package ghappauth

import "errors"

var (
	// ErrPrivateKeyInvalid is returned when the PEM private key cannot be parsed.
	ErrPrivateKeyInvalid = errors.New("ghappauth: invalid private key")

	// ErrPrivateKeyMissing is returned when neither PrivateKeyPath nor PrivateKey is set.
	ErrPrivateKeyMissing = errors.New("ghappauth: private key not provided")

	// ErrTokenRefreshFailed is returned when an installation token cannot be obtained.
	ErrTokenRefreshFailed = errors.New("ghappauth: token refresh failed")

	// ErrWebhookSignatureMismatch is returned when webhook HMAC validation fails.
	ErrWebhookSignatureMismatch = errors.New("ghappauth: webhook signature mismatch")

	// ErrWebhookSignatureMissing is returned when the signature header is absent.
	ErrWebhookSignatureMissing = errors.New("ghappauth: webhook signature missing")

	// ErrInstallationNotFound is returned when GitHub returns 404 for an installation.
	ErrInstallationNotFound = errors.New("ghappauth: installation not found")
)
```

Create `ghappauth.go`:

```go
package ghappauth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"
)

// Config holds the configuration for a GitHub App.
type Config struct {
	AppID          int64
	PrivateKeyPath string // path to PEM file (mutually exclusive with PrivateKey)
	PrivateKey     []byte // raw PEM bytes (mutually exclusive with PrivateKeyPath)
	WebhookSecret  string // optional, only needed for webhook verification
	BaseURL        string // optional, defaults to "https://api.github.com"
}

// tokenEntry caches an installation access token.
type tokenEntry struct {
	token   string
	expires time.Time
}

// App is the main handle for GitHub App authentication.
type App struct {
	appID   int64
	key     *rsa.PrivateKey
	secret  string
	baseURL string

	mu     sync.RWMutex
	tokens map[int64]tokenEntry // installationID -> cached token
}

// New creates a new App from the given config.
func New(cfg Config) (*App, error) {
	keyData, err := loadKey(cfg)
	if err != nil {
		return nil, err
	}

	key, err := parsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	return &App{
		appID:   cfg.AppID,
		key:     key,
		secret:  cfg.WebhookSecret,
		baseURL: baseURL,
		tokens:  make(map[int64]tokenEntry),
	}, nil
}

func loadKey(cfg Config) ([]byte, error) {
	if len(cfg.PrivateKey) > 0 {
		return cfg.PrivateKey, nil
	}
	if cfg.PrivateKeyPath != "" {
		data, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPrivateKeyInvalid, err)
		}
		return data, nil
	}
	return nil, ErrPrivateKeyMissing
}

func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrPrivateKeyInvalid
	}

	// Try PKCS1 first, then PKCS8.
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPrivateKeyInvalid, err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: not an RSA key", ErrPrivateKeyInvalid)
	}
	return key, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test -v -run TestNew`
Expected: PASS — all 4 TestNew tests pass

**Step 6: Commit**

```bash
git add go.mod ghappauth.go errors.go ghappauth_test.go
git commit -m "feat: add Config, New(), errors, and PEM key parsing"
```

---

### Task 2: JWT Generation

**Files:**
- Create: `jwt.go`
- Create: `jwt_test.go`

**Step 1: Write the failing test**

Create `jwt_test.go`:

```go
package ghappauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateJWT(t *testing.T) {
	key := testKey(t)
	token, err := generateJWT(12345, key)
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	// Decode header.
	hdr, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(hdr, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "RS256" {
		t.Errorf("alg = %q, want RS256", header["alg"])
	}
	if header["typ"] != "JWT" {
		t.Errorf("typ = %q, want JWT", header["typ"])
	}

	// Decode payload.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	iss, ok := claims["iss"].(string)
	if !ok || iss != "12345" {
		t.Errorf("iss = %v, want \"12345\"", claims["iss"])
	}

	// exp should be ~10 minutes from now.
	exp := int64(claims["exp"].(float64))
	now := time.Now().Unix()
	if exp < now+500 || exp > now+700 {
		t.Errorf("exp = %d, want ~%d (10 min from now)", exp, now+600)
	}
}

func TestGenerateJWT_VerifySignature(t *testing.T) {
	key := testKey(t)
	token, err := generateJWT(99, key)
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}

	// Verify the signature using the public key.
	if err := verifyJWTSignature(token, &key.PublicKey); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

// verifyJWTSignature is a test helper that verifies an RS256 JWT.
func verifyJWTSignature(token string, pub *rsa.PublicKey) error {
	parts := strings.Split(token, ".")
	signed := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}

	import (
		"crypto"
		"crypto/sha256"
	)

	hash := sha256.Sum256([]byte(signed))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig)
}
```

Wait — Go doesn't allow imports inside functions. Let me fix the test file:

Create `jwt_test.go`:

```go
package ghappauth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateJWT(t *testing.T) {
	key := testKey(t)
	token, err := generateJWT(12345, key)
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}

	// Decode header.
	hdr, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(hdr, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "RS256" {
		t.Errorf("alg = %q, want RS256", header["alg"])
	}
	if header["typ"] != "JWT" {
		t.Errorf("typ = %q, want JWT", header["typ"])
	}

	// Decode payload.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	iss, ok := claims["iss"].(string)
	if !ok || iss != "12345" {
		t.Errorf("iss = %v, want \"12345\"", claims["iss"])
	}

	// exp should be ~10 minutes from now.
	exp := int64(claims["exp"].(float64))
	now := time.Now().Unix()
	if exp < now+500 || exp > now+700 {
		t.Errorf("exp = %d, want ~%d (10 min from now)", exp, now+600)
	}
}

func TestGenerateJWT_VerifySignature(t *testing.T) {
	key := testKey(t)
	token, err := generateJWT(99, key)
	if err != nil {
		t.Fatalf("generateJWT() error: %v", err)
	}

	parts := strings.Split(token, ".")
	signed := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}

	hash := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sig); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestGenerateJWT`
Expected: FAIL — `generateJWT` undefined

**Step 3: Write minimal implementation**

Create `jwt.go`:

```go
package ghappauth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// generateJWT creates an RS256-signed JWT for GitHub App authentication.
// The token is valid for 10 minutes per GitHub's spec.
func generateJWT(appID int64, key *rsa.PrivateKey) (string, error) {
	now := time.Now()

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}
	claims := map[string]interface{}{
		"iat": now.Add(-60 * time.Second).Unix(), // 60s clock skew buffer
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": strconv.FormatInt(appID, 10),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signed := headerB64 + "." + claimsB64

	hash := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signed + "." + sigB64, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestGenerateJWT`
Expected: PASS

**Step 5: Commit**

```bash
git add jwt.go jwt_test.go
git commit -m "feat: add RS256 JWT generation for GitHub App auth"
```

---

### Task 3: Installation Token Exchange + Caching

**Files:**
- Create: `installation.go`
- Create: `installation_test.go`

**Step 1: Write the failing test**

Create `installation_test.go`:

```go
package ghappauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstallationToken(t *testing.T) {
	// Mock GitHub API.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path: POST /app/installations/{id}/access_tokens
		if !strings.HasPrefix(r.URL.Path, "/app/installations/") || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify JWT in Authorization header.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("missing Bearer token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := map[string]interface{}{
			"token":      "ghs_test_token_abc123",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, err := New(Config{
		AppID:          12345,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	token, expiry, err := app.InstallationToken(context.Background(), 67890)
	if err != nil {
		t.Fatalf("InstallationToken() error: %v", err)
	}
	if token != "ghs_test_token_abc123" {
		t.Errorf("token = %q, want ghs_test_token_abc123", token)
	}
	if expiry.IsZero() {
		t.Error("expiry is zero")
	}
}

func TestInstallationToken_Caching(t *testing.T) {
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		resp := map[string]interface{}{
			"token":      "ghs_cached_token",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, err := New(Config{
		AppID:          12345,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First call — hits API.
	_, _, err = app.InstallationToken(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Second call — should use cache.
	_, _, err = app.InstallationToken(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}

	if callCount.Load() != 1 {
		t.Errorf("API called %d times, want 1 (cached)", callCount.Load())
	}
}

func TestInstallationToken_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, err := New(Config{
		AppID:          12345,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = app.InstallationToken(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestInstallationClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"token":      "ghs_client_token",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, err := New(Config{
		AppID:          12345,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	client, err := app.InstallationClient(context.Background(), 100)
	if err != nil {
		t.Fatalf("InstallationClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestInstallation`
Expected: FAIL — `InstallationToken` undefined

**Step 3: Write minimal implementation**

Create `installation.go`:

```go
package ghappauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const tokenExpiryBuffer = 5 * time.Minute

// InstallationToken returns an access token for the given installation.
// Tokens are cached in-memory and refreshed 5 minutes before expiry.
func (a *App) InstallationToken(ctx context.Context, installationID int64) (string, time.Time, error) {
	// Check cache.
	a.mu.RLock()
	if entry, ok := a.tokens[installationID]; ok {
		if time.Now().Before(entry.expires.Add(-tokenExpiryBuffer)) {
			a.mu.RUnlock()
			return entry.token, entry.expires, nil
		}
	}
	a.mu.RUnlock()

	// Exchange JWT for installation token.
	jwt, err := generateJWT(a.appID, a.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrTokenRefreshFailed, err)
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", a.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrTokenRefreshFailed, err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrTokenRefreshFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", time.Time{}, ErrInstallationNotFound
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("%w: status %d: %s", ErrTokenRefreshFailed, resp.StatusCode, body)
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, fmt.Errorf("%w: decode response: %v", ErrTokenRefreshFailed, err)
	}

	// Cache the token.
	a.mu.Lock()
	a.tokens[installationID] = tokenEntry{
		token:   result.Token,
		expires: result.ExpiresAt,
	}
	a.mu.Unlock()

	return result.Token, result.ExpiresAt, nil
}

// InstallationClient returns an *http.Client that authenticates requests
// with the installation's access token.
func (a *App) InstallationClient(ctx context.Context, installationID int64) (*http.Client, error) {
	// Verify we can get a token before returning the client.
	if _, _, err := a.InstallationToken(ctx, installationID); err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: a.Transport(installationID),
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestInstallation`
Expected: FAIL — `Transport` undefined (we'll stub it). Actually, `Transport` is in Task 4. We need to either inline a simple transport or reorder. Let's use a simpler `InstallationClient` that creates a static token client:

Replace `InstallationClient` with:

```go
func (a *App) InstallationClient(ctx context.Context, installationID int64) (*http.Client, error) {
	token, _, err := a.InstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &staticTokenTransport{token: token},
	}, nil
}

type staticTokenTransport struct {
	token string
}

func (t *staticTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "token "+t.token)
	return http.DefaultTransport.RoundTrip(req2)
}
```

Then in Task 4, we upgrade `InstallationClient` to use the auto-refreshing transport.

Run: `go test -v -run TestInstallation`
Expected: PASS

**Step 5: Commit**

```bash
git add installation.go installation_test.go
git commit -m "feat: add installation token exchange with in-memory caching"
```

---

### Task 4: Webhook Verification + Middleware

**Files:**
- Create: `webhook.go`
- Create: `webhook_test.go`

**Step 1: Write the failing test**

Create `webhook_test.go`:

```go
package ghappauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func computeHMAC(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhook_Valid(t *testing.T) {
	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		WebhookSecret:  "test-secret",
	})

	payload := `{"action":"labeled"}`
	sig := computeHMAC("test-secret", payload)

	if err := app.VerifyWebhook(sig, []byte(payload)); err != nil {
		t.Fatalf("VerifyWebhook() error: %v", err)
	}
}

func TestVerifyWebhook_Invalid(t *testing.T) {
	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		WebhookSecret:  "test-secret",
	})

	payload := `{"action":"labeled"}`
	badSig := computeHMAC("wrong-secret", payload)

	err := app.VerifyWebhook(badSig, []byte(payload))
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestVerifyWebhook_Missing(t *testing.T) {
	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		WebhookSecret:  "test-secret",
	})

	err := app.VerifyWebhook("", []byte("payload"))
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestWebhookMiddleware(t *testing.T) {
	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		WebhookSecret:  "mw-secret",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := app.WebhookMiddleware(inner)

	t.Run("valid signature passes through", func(t *testing.T) {
		body := `{"test":true}`
		sig := computeHMAC("mw-secret", body)

		req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", sig)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("invalid signature returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader("body"))
		req.Header.Set("X-Hub-Signature-256", "sha256=bad")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("missing signature returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader("body"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestVerifyWebhook -run TestWebhookMiddleware`
Expected: FAIL — `VerifyWebhook` undefined

**Step 3: Write minimal implementation**

Create `webhook.go`:

```go
package ghappauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

// VerifyWebhook validates a GitHub webhook payload against its HMAC-SHA256 signature.
func (a *App) VerifyWebhook(signature string, payload []byte) error {
	if signature == "" {
		return ErrWebhookSignatureMissing
	}

	sig := strings.TrimPrefix(signature, "sha256=")
	decoded, err := hex.DecodeString(sig)
	if err != nil {
		return ErrWebhookSignatureMismatch
	}

	mac := hmac.New(sha256.New, []byte(a.secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	if !hmac.Equal(decoded, expected) {
		return ErrWebhookSignatureMismatch
	}
	return nil
}

// WebhookMiddleware returns an http.Handler that verifies the X-Hub-Signature-256
// header before passing the request to the next handler.
func (a *App) WebhookMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		sig := r.Header.Get("X-Hub-Signature-256")
		if err := a.VerifyWebhook(sig, body); err != nil {
			http.Error(w, "invalid signature", http.StatusForbidden)
			return
		}

		// Re-create the body for downstream handlers.
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		next.ServeHTTP(w, r)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run "TestVerifyWebhook|TestWebhookMiddleware"`
Expected: PASS

**Step 5: Commit**

```bash
git add webhook.go webhook_test.go
git commit -m "feat: add webhook HMAC-SHA256 verification and middleware"
```

---

### Task 5: Auto-Refreshing Transport

**Files:**
- Create: `transport.go`
- Create: `transport_test.go`
- Modify: `installation.go` — update `InstallationClient` to use `Transport`

**Step 1: Write the failing test**

Create `transport_test.go`:

```go
package ghappauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTransport_InjectsAuthHeader(t *testing.T) {
	// Mock GitHub token API.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"token":      "ghs_transport_token",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Mock target API — captures the Authorization header.
	var gotAuth string
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		BaseURL:        tokenServer.URL,
	})

	// Pre-populate cache so Transport doesn't need to call the token server.
	app.mu.Lock()
	app.tokens[42] = tokenEntry{
		token:   "ghs_transport_token",
		expires: time.Now().Add(1 * time.Hour),
	}
	app.mu.Unlock()

	client := &http.Client{Transport: app.Transport(42)}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", targetServer.URL+"/repos", nil)
	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if gotAuth != "token ghs_transport_token" {
		t.Errorf("Authorization = %q, want \"token ghs_transport_token\"", gotAuth)
	}
}

func TestTransport_RefreshesExpiredToken(t *testing.T) {
	var tokenCalls atomic.Int32

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		resp := map[string]interface{}{
			"token":      "ghs_refreshed",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	var gotAuth string
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		BaseURL:        tokenServer.URL,
	})

	// Set an expired token in cache.
	app.mu.Lock()
	app.tokens[42] = tokenEntry{
		token:   "ghs_expired",
		expires: time.Now().Add(-1 * time.Hour), // expired
	}
	app.mu.Unlock()

	client := &http.Client{Transport: app.Transport(42)}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", targetServer.URL+"/test", nil)
	_, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if gotAuth != "token ghs_refreshed" {
		t.Errorf("Authorization = %q, want \"token ghs_refreshed\"", gotAuth)
	}
	if tokenCalls.Load() != 1 {
		t.Errorf("token API called %d times, want 1", tokenCalls.Load())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestTransport`
Expected: FAIL — `Transport` undefined

**Step 3: Write minimal implementation**

Create `transport.go`:

```go
package ghappauth

import (
	"context"
	"net/http"
)

// Transport returns an http.RoundTripper that injects an installation
// access token into every request, refreshing automatically when expired.
func (a *App) Transport(installationID int64) http.RoundTripper {
	return &installationTransport{
		app:            a,
		installationID: installationID,
	}
}

type installationTransport struct {
	app            *App
	installationID int64
}

func (t *installationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, _, err := t.app.InstallationToken(req.Context(), t.installationID)
	if err != nil {
		return nil, err
	}

	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "token "+token)
	return http.DefaultTransport.RoundTrip(req2)
}
```

Now update `installation.go` — replace the `staticTokenTransport` `InstallationClient` with the proper transport:

```go
// InstallationClient returns an *http.Client that authenticates requests
// with an auto-refreshing installation access token.
func (a *App) InstallationClient(ctx context.Context, installationID int64) (*http.Client, error) {
	if _, _, err := a.InstallationToken(ctx, installationID); err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: a.Transport(installationID),
	}, nil
}
```

Remove the `staticTokenTransport` from `installation.go`.

**Step 4: Run test to verify it passes**

Run: `go test -v -run "TestTransport|TestInstallation"`
Expected: PASS

**Step 5: Commit**

```bash
git add transport.go transport_test.go installation.go
git commit -m "feat: add auto-refreshing HTTP transport for installation tokens"
```

---

### Task 6: Git Credential Helper

**Files:**
- Create: `credential.go`
- Create: `credential_test.go`

**Step 1: Write the failing test**

Create `credential_test.go`:

```go
package ghappauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestGitCredentialHelper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("credential helper uses shell scripts, skipping on Windows")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"token":      "ghs_cred_token",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})

	scriptPath, cleanup, err := app.GitCredentialHelper(context.Background(), 42)
	if err != nil {
		t.Fatalf("GitCredentialHelper() error: %v", err)
	}
	defer cleanup()

	// Verify the script exists and is executable.
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("stat script: %v", err)
	}
	if info.Mode()&0100 == 0 {
		t.Error("script is not executable")
	}

	// Execute the script and verify it prints the token.
	out, err := exec.Command(scriptPath).Output()
	if err != nil {
		t.Fatalf("exec script: %v", err)
	}
	if string(out) != "ghs_cred_token" {
		t.Errorf("script output = %q, want \"ghs_cred_token\"", string(out))
	}
}

func TestGitCredentialHelper_Cleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("credential helper uses shell scripts, skipping on Windows")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"token":      "ghs_cleanup_token",
			"expires_at": time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	key := testKey(t)
	path := testPEMFile(t, key)

	app, _ := New(Config{
		AppID:          1,
		PrivateKeyPath: path,
		BaseURL:        ts.URL,
	})

	scriptPath, cleanup, err := app.GitCredentialHelper(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}

	cleanup()

	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Error("script file still exists after cleanup")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestGitCredentialHelper`
Expected: FAIL — `GitCredentialHelper` undefined

**Step 3: Write minimal implementation**

Create `credential.go`:

```go
package ghappauth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// GitCredentialHelper creates a temporary GIT_ASKPASS script that returns
// the current installation token. Call cleanup() to remove the script.
//
// Usage:
//
//	scriptPath, cleanup, err := app.GitCredentialHelper(ctx, installationID)
//	defer cleanup()
//	cmd := exec.Command("git", "clone", repoURL)
//	cmd.Env = append(os.Environ(), "GIT_ASKPASS="+scriptPath)
func (a *App) GitCredentialHelper(ctx context.Context, installationID int64) (string, func(), error) {
	token, _, err := a.InstallationToken(ctx, installationID)
	if err != nil {
		return "", nil, err
	}

	dir, err := os.MkdirTemp("", "ghappauth-cred-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	scriptPath := filepath.Join(dir, "git-askpass.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\n", token)

	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("write credential script: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return scriptPath, cleanup, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestGitCredentialHelper`
Expected: PASS

**Step 5: Commit**

```bash
git add credential.go credential_test.go
git commit -m "feat: add GIT_ASKPASS credential helper for git clone/push"
```

---

### Task 7: README + LICENSE + Final Polish

**Files:**
- Create: `README.md`
- Create: `LICENSE`

**Step 1: Create README**

Create `README.md`:

```markdown
# ghappauth

Reusable Go module for GitHub App authentication.

## Install

` ` `bash
go get github.com/mandadapu/ghappauth
` ` `

## Usage

` ` `go
import "github.com/mandadapu/ghappauth"

// Create app from PEM file.
app, err := ghappauth.New(ghappauth.Config{
    AppID:          12345,
    PrivateKeyPath: "/path/to/app.pem",
    WebhookSecret:  "whsec_...", // optional
})

// Get authenticated HTTP client for an installation.
client, err := app.InstallationClient(ctx, installationID)

// Get raw token (for passing to executors, K8s Secrets, etc.).
token, expiry, err := app.InstallationToken(ctx, installationID)

// Verify webhook signature.
err = app.VerifyWebhook(signature, payload)

// Use as HTTP middleware.
mux.Handle("/webhooks", app.WebhookMiddleware(next))

// Git credential helper for clone/push.
scriptPath, cleanup, err := app.GitCredentialHelper(ctx, installationID)
defer cleanup()
cmd := exec.Command("git", "clone", repoURL)
cmd.Env = append(os.Environ(), "GIT_ASKPASS="+scriptPath)
` ` `

## Features

- **JWT generation** — RS256 JWTs from PEM private key, no external JWT library
- **Installation tokens** — Exchange + in-memory caching with automatic refresh
- **Webhook verification** — HMAC-SHA256 validation + drop-in HTTP middleware
- **Auto-refreshing transport** — `http.RoundTripper` that injects fresh tokens
- **Git credential helper** — `GIT_ASKPASS` script for clone/push operations
- **Zero heavy dependencies** — stdlib crypto + net/http only

## License

MIT
```

**Step 2: Create LICENSE (MIT)**

Create `LICENSE` with standard MIT license text.

**Step 3: Run all tests**

Run: `go test -v -race ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add README.md LICENSE
git commit -m "docs: add README and MIT license"
```

**Step 5: Tag and push**

```bash
git tag v0.1.0
git push origin main --tags
```

---

### Task 8: Wire ghappauth into NeuralForge

**Files:**
- Modify: `internal/app/app.go` — import ghappauth, create App, wire webhook middleware and GitHub client
- Modify: `internal/app/webhook.go` — remove manual HMAC verification (replaced by ghappauth middleware)
- Modify: `go.mod` — add `github.com/mandadapu/ghappauth` dependency

**Step 1: Add dependency**

```bash
cd /Users/suryamandadapu/src/neuralforge
go get github.com/mandadapu/ghappauth@v0.1.0
```

**Step 2: Update `internal/app/app.go`**

In `New()`, after opening the store, create the ghappauth App:

```go
import "github.com/mandadapu/ghappauth"

// In New(), after store setup:
var ghApp *ghappauth.App
if cfg.GitHub.PrivateKeyPath != "" {
    var err error
    ghApp, err = ghappauth.New(ghappauth.Config{
        AppID:          cfg.GitHub.AppID,
        PrivateKeyPath: cfg.GitHub.PrivateKeyPath,
        WebhookSecret:  cfg.GitHub.WebhookSecret,
    })
    if err != nil {
        s.Close()
        return nil, fmt.Errorf("init github app auth: %w", err)
    }
}
```

Replace webhook handler setup:

```go
// If ghApp is configured, use its middleware for webhook verification.
// Otherwise fall back to the existing manual HMAC handler.
if ghApp != nil {
    mux.Handle("/webhooks/github", ghApp.WebhookMiddleware(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Body already verified by middleware.
            body, _ := io.ReadAll(r.Body)
            eventType := r.Header.Get("X-GitHub-Event")
            a.handleEvent(eventType, body)
            w.WriteHeader(http.StatusOK)
        }),
    ))
} else {
    mux.Handle("/webhooks/github", NewWebhookHandler(cfg.GitHub.WebhookSecret, a.handleEvent))
}
```

In `buildJobHandler`, pass ghApp for creating authenticated clients:

```go
// In the job handler closure, get installation client:
if ghApp != nil {
    // TODO: look up installationID from repo. For now, use env var.
    installID := int64(envInt("GITHUB_INSTALLATION_ID", 0))
    if installID > 0 {
        httpClient, err := ghApp.InstallationClient(ctx, installID)
        if err == nil {
            ghClient := github.NewClient(httpClient)
            // Now wire PR, review, merge, deploy stages with ghClient
        }
    }
}
```

**Step 3: Run NeuralForge tests**

Run: `cd /Users/suryamandadapu/src/neuralforge && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add go.mod go.sum internal/app/app.go
git commit -m "feat: integrate ghappauth for GitHub App authentication"
```
