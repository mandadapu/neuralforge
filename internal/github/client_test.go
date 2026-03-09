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

// testTransport redirects all HTTP requests to the given test server,
// preserving path and query but replacing scheme and host.
type testTransport struct {
	serverURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base, err := url.Parse(t.serverURL)
	if err != nil {
		return nil, err
	}
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = base.Scheme
	newReq.URL.Host = base.Host
	return http.DefaultTransport.RoundTrip(newReq)
}

func newTestClient(server *httptest.Server) *Client {
	httpClient := &http.Client{Transport: &testTransport{serverURL: server.URL}}
	return NewClient(httpClient)
}

func TestCreatePR_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"number":   42,
			"html_url": "https://github.com/owner/repo/pull/42",
		})
	}))
	defer server.Close()

	c := newTestClient(server)
	num, htmlURL, err := c.CreatePR(context.Background(), "owner", "repo", "title", "body", "head", "main")

	require.NoError(t, err)
	assert.Equal(t, 42, num)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", htmlURL)
}

func TestCreatePR_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Validation Failed"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, _, err := c.CreatePR(context.Background(), "owner", "repo", "title", "body", "head", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")
}

func TestCommentOnIssue_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "body": "test"})
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.CommentOnIssue(context.Background(), "owner", "repo", 1, "hello")

	require.NoError(t, err)
}

func TestCommentOnIssue_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.CommentOnIssue(context.Background(), "owner", "repo", 999, "hi")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment on issue")
}

func TestMergePR_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"sha": "abc123", "merged": true})
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.MergePR(context.Background(), "owner", "repo", 42, "merge commit")

	require.NoError(t, err)
}

func TestMergePR_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"message":"Method Not Allowed"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.MergePR(context.Background(), "owner", "repo", 42, "msg")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge PR")
}

func TestCreateReview_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "state": "APPROVED"})
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.CreateReview(context.Background(), "owner", "repo", 42, "looks good", "APPROVE")

	require.NoError(t, err)
}

func TestGetFileContent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// "hello world" base64-encoded
		json.NewEncoder(w).Encode(map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  "aGVsbG8gd29ybGQ=\n",
			"name":     "test.txt",
			"path":     "path/to/test.txt",
			"sha":      "abc123",
			"size":     11,
		})
	}))
	defer server.Close()

	c := newTestClient(server)
	content, err := c.GetFileContent(context.Background(), "owner", "repo", "path/to/test.txt", "main")

	require.NoError(t, err)
	assert.Equal(t, "hello world", content)
}

func TestGetFileContent_DirectoryPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a JSON array (directory listing) — go-github sets fileContent=nil
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]any{
			{"type": "file", "name": "a.txt", "path": "dir/a.txt", "sha": "x"},
		})
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.GetFileContent(context.Background(), "owner", "repo", "dir", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestGetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.GetFileContent(context.Background(), "owner", "repo", "missing.txt", "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get file content")
}
