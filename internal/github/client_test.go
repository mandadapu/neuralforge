package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client pointed at a test HTTP server.
// It overrides the go-github BaseURL to route all requests to the test server.
func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	c := NewClient(http.DefaultClient)

	// Override go-github's BaseURL to the test server (accessible since we are in package github).
	base, err := url.Parse(ts.URL + "/")
	require.NoError(t, err)
	c.gh.BaseURL = base
	c.gh.UploadURL = base

	return c
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestCreatePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		writeJSON(w, map[string]any{
			"number":   42,
			"html_url": "https://github.com/owner/repo/pull/42",
		})
	})

	c := newTestClient(t, mux)
	prNum, prURL, err := c.CreatePR(context.Background(), "owner", "repo", "title", "body", "head-branch", "main")
	require.NoError(t, err)
	assert.Equal(t, 42, prNum)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", prURL)
}

func TestCreatePRError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	})

	c := newTestClient(t, mux)
	_, _, err := c.CreatePR(context.Background(), "owner", "repo", "title", "body", "head", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")
}

func TestCommentOnIssue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		writeJSON(w, map[string]any{"id": 1, "body": "hello"})
	})

	c := newTestClient(t, mux)
	err := c.CommentOnIssue(context.Background(), "owner", "repo", 5, "hello")
	require.NoError(t, err)
}

func TestCommentOnIssueError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})

	c := newTestClient(t, mux)
	err := c.CommentOnIssue(context.Background(), "owner", "repo", 5, "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment on issue")
}

func TestMergePR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/7/merge", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		writeJSON(w, map[string]any{"merged": true, "message": "Pull Request successfully merged"})
	})

	c := newTestClient(t, mux)
	err := c.MergePR(context.Background(), "owner", "repo", 7, "merge commit message")
	require.NoError(t, err)
}

func TestMergePRError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/7/merge", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
	})

	c := newTestClient(t, mux)
	err := c.MergePR(context.Background(), "owner", "repo", 7, "msg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge PR")
}

func TestCreateReview(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/3/reviews", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		writeJSON(w, map[string]any{"id": 1, "state": "APPROVED"})
	})

	c := newTestClient(t, mux)
	err := c.CreateReview(context.Background(), "owner", "repo", 3, "LGTM", "APPROVE")
	require.NoError(t, err)
}

func TestCreateReviewError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/3/reviews", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Forbidden"}`, http.StatusForbidden)
	})

	c := newTestClient(t, mux)
	err := c.CreateReview(context.Background(), "owner", "repo", 3, "body", "APPROVE")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create review")
}

func TestGetFileContent(t *testing.T) {
	// "hello world" base64-encoded with a trailing newline (standard go base64 output)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/contents/README.md", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "main", r.URL.Query().Get("ref"))
		writeJSON(w, map[string]any{
			"type":     "file",
			"name":     "README.md",
			"encoding": "base64",
			// "hello world" base64-encoded
			"content": "aGVsbG8gd29ybGQ=\n",
			"sha":     "abc123",
		})
	})

	c := newTestClient(t, mux)
	content, err := c.GetFileContent(context.Background(), "owner", "repo", "README.md", "main")
	require.NoError(t, err)
	assert.Equal(t, "hello world", content)
}

func TestGetFileContentError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/contents/missing.txt", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})

	c := newTestClient(t, mux)
	_, err := c.GetFileContent(context.Background(), "owner", "repo", "missing.txt", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get file content")
}
