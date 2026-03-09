package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v60/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client pointed at a test HTTP server.
func newTestClient(t *testing.T, mux *http.ServeMux) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &Client{gh: gh.NewClient(nil)}
	base, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	c.gh.BaseURL = base
	c.gh.UploadURL = base

	return c, srv
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestClient_CreatePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "fix: add feature", body["title"])

		writeJSON(w, http.StatusCreated, map[string]any{
			"number":   42,
			"html_url": "https://github.com/owner/repo/pull/42",
		})
	})

	client, _ := newTestClient(t, mux)

	prNum, prURL, err := client.CreatePR(context.Background(),
		"owner", "repo", "fix: add feature", "PR body", "feature-branch", "main")

	require.NoError(t, err)
	assert.Equal(t, 42, prNum)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", prURL)
}

func TestClient_CreatePR_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Validation failed",
		})
	})

	client, _ := newTestClient(t, mux)

	_, _, err := client.CreatePR(context.Background(),
		"owner", "repo", "title", "body", "head", "base")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")
}

func TestClient_CommentOnIssue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "test comment", body["body"])

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":   1,
			"body": "test comment",
		})
	})

	client, _ := newTestClient(t, mux)

	err := client.CommentOnIssue(context.Background(), "owner", "repo", 5, "test comment")

	require.NoError(t, err)
}

func TestClient_CommentOnIssue_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]any{"message": "Forbidden"})
	})

	client, _ := newTestClient(t, mux)

	err := client.CommentOnIssue(context.Background(), "owner", "repo", 5, "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment on issue")
}

func TestClient_MergePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/7/merge", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		writeJSON(w, http.StatusOK, map[string]any{
			"sha":     "abc123",
			"merged":  true,
			"message": "Pull Request successfully merged",
		})
	})

	client, _ := newTestClient(t, mux)

	err := client.MergePR(context.Background(), "owner", "repo", 7, "merge commit message")

	require.NoError(t, err)
}

func TestClient_MergePR_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/7/merge", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"message": "Pull Request is not mergeable",
		})
	})

	client, _ := newTestClient(t, mux)

	err := client.MergePR(context.Background(), "owner", "repo", 7, "message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge PR")
}

func TestClient_CreateReview(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/3/reviews", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "APPROVE", body["event"])

		writeJSON(w, http.StatusOK, map[string]any{
			"id":    100,
			"state": "APPROVED",
		})
	})

	client, _ := newTestClient(t, mux)

	err := client.CreateReview(context.Background(), "owner", "repo", 3, "Looks good", "APPROVE")

	require.NoError(t, err)
}

func TestClient_CreateReview_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/3/reviews", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]any{"message": "Forbidden"})
	})

	client, _ := newTestClient(t, mux)

	err := client.CreateReview(context.Background(), "owner", "repo", 3, "body", "APPROVE")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create review")
}

func TestClient_GetFileContent(t *testing.T) {
	// "hello world" base64 encoded
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "main", r.URL.Query().Get("ref"))

		writeJSON(w, http.StatusOK, map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  "aGVsbG8gd29ybGQ=\n", // "hello world" base64
			"name":     "README.md",
		})
	})

	client, _ := newTestClient(t, mux)

	content, err := client.GetFileContent(context.Background(), "owner", "repo", "README.md", "main")

	require.NoError(t, err)
	assert.Equal(t, "hello world", content)
}

func TestClient_GetFileContent_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/contents/missing.md", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	})

	client, _ := newTestClient(t, mux)

	_, err := client.GetFileContent(context.Background(), "owner", "repo", "missing.md", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get file content")
}
