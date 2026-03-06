# ghappauth — Reusable GitHub App Auth Module Design

## Goal

Self-contained Go module for GitHub App authentication, importable by NeuralForge, NeuralWarden, and future apps via `go get github.com/mandadapu/ghappauth`.

## Module

- **Repo:** `github.com/mandadapu/ghappauth`
- **Import:** `go get github.com/mandadapu/ghappauth`
- Minimal dependencies: `crypto` stdlib + `golang.org/x/oauth2`

## Architecture

Flat package structure — all exported types in the root package:

```
ghappauth/
├── ghappauth.go      # Config, New(), App struct
├── jwt.go            # RS256 JWT generation from PEM private key
├── installation.go   # Installation token exchange + in-memory caching
├── webhook.go        # HMAC-SHA256 webhook signature verification + middleware
├── transport.go      # http.RoundTripper with auto-refreshing installation tokens
├── credential.go     # GIT_ASKPASS helper for git clone/push
├── errors.go         # Typed error values
├── ghappauth_test.go # Unit tests (fake RSA keys, httptest)
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## Components

### 1. JWT Signer (`jwt.go`)

Generates RS256 JWTs from a PEM private key. 10-minute expiry per GitHub spec. Used internally by the installation token manager.

- `generateJWT(appID int64, key *rsa.PrivateKey) (string, error)`
- No external JWT library — uses `crypto/rsa`, `encoding/json`, `encoding/base64`

### 2. Installation Token Manager (`installation.go`)

Exchanges JWT for installation access tokens via `POST /app/installations/{id}/access_tokens`. Caches tokens in-memory with 5-minute early expiry buffer. Thread-safe via `sync.RWMutex`.

- `(a *App) InstallationToken(ctx context.Context, installationID int64) (string, time.Time, error)`
- `(a *App) InstallationClient(ctx context.Context, installationID int64) (*http.Client, error)`
- Optional repository and permission scoping

### 3. Webhook Verifier (`webhook.go`)

Validates `X-Hub-Signature-256` headers using HMAC-SHA256.

- `(a *App) VerifyWebhook(signature string, payload []byte) error`
- `(a *App) WebhookMiddleware(next http.Handler) http.Handler`

### 4. Auto-Refreshing Transport (`transport.go`)

`http.RoundTripper` that injects `Authorization: token <installation-token>` on every request, auto-refreshing when expired.

- `(a *App) Transport(installationID int64) http.RoundTripper`

### 5. Git Credential Helper (`credential.go`)

Generates a temporary `GIT_ASKPASS` script that returns the current installation token. For use with `git clone`/`git push` in executor contexts (Docker, K8s).

- `(a *App) GitCredentialHelper(ctx context.Context, installationID int64) (scriptPath string, cleanup func(), err error)`

## API Surface

```go
// Core constructor
app, err := ghappauth.New(ghappauth.Config{
    AppID:          12345,
    PrivateKeyPath: "/path/to/app.pem",  // file path
    // OR
    PrivateKey:     pemBytes,            // raw PEM bytes
    WebhookSecret:  "whsec_...",         // optional
})

// Authenticated http.Client for an installation
client, err := app.InstallationClient(ctx, installationID)

// Raw installation token (for K8s Secrets, env vars, etc.)
token, expiry, err := app.InstallationToken(ctx, installationID)

// Webhook verification
err := app.VerifyWebhook(signature, payload)

// HTTP middleware
mux.Handle("/webhooks", app.WebhookMiddleware(next))

// Git credential helper
scriptPath, cleanup, err := app.GitCredentialHelper(ctx, installationID)
defer cleanup()
```

## Consumer Integration

### NeuralForge (Go)

```go
import "github.com/mandadapu/ghappauth"

ghApp, err := ghappauth.New(ghappauth.Config{
    AppID:          cfg.GitHub.AppID,
    PrivateKeyPath: cfg.GitHub.PrivateKeyPath,
    WebhookSecret:  cfg.GitHub.WebhookSecret,
})

// Webhook handler — replace manual HMAC in app/webhook.go
mux.Handle("/webhooks/github", ghApp.WebhookMiddleware(webhookHandler))

// Pipeline stages — create authenticated GitHub client
httpClient, _ := ghApp.InstallationClient(ctx, installID)
ghClient := github.NewClient(httpClient)

// K8s executor — pass token for git clone/push
token, _, _ := ghApp.InstallationToken(ctx, installID)
```

### NeuralWarden (Python)

Can use the Go module via:
- CLI wrapper: `ghappauth token --app-id=X --key=Y --installation=Z`
- Or continue using PATs and migrate later

## Errors

```go
var (
    ErrPrivateKeyInvalid      = errors.New("ghappauth: invalid private key")
    ErrTokenExpired           = errors.New("ghappauth: token refresh failed")
    ErrWebhookSignatureMismatch = errors.New("ghappauth: webhook signature mismatch")
    ErrWebhookSignatureMissing  = errors.New("ghappauth: webhook signature missing")
    ErrInstallationNotFound   = errors.New("ghappauth: installation not found")
)
```

## Testing

- Unit tests with fake RSA keys generated at test time
- `httptest.Server` to mock GitHub API responses
- Integration test tag (`//go:build integration`) for real GitHub API
- Zero dependency on any specific GitHub client library

## Configuration (for consumers)

| Env Var | Description |
|---------|-------------|
| `GITHUB_APP_ID` | GitHub App ID |
| `GITHUB_PRIVATE_KEY_PATH` | Path to PEM private key file |
| `GITHUB_WEBHOOK_SECRET` | Webhook signing secret |

Note: The module itself does NOT read env vars. Consumers pass config explicitly via `ghappauth.Config{}`.
