package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v60/github"
)

// Client wraps the go-github client with convenience methods.
type Client struct {
	gh *gh.Client
}

// NewClient creates a new GitHub API client using the provided http.Client
// (which should carry authentication, e.g. via oauth2.NewClient).
func NewClient(httpClient *http.Client) *Client {
	return &Client{gh: gh.NewClient(httpClient)}
}

// CreatePR opens a new pull request and returns the PR number and URL.
func (c *Client) CreatePR(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, owner, repo, &gh.NewPullRequest{
		Title: gh.String(title),
		Body:  gh.String(body),
		Head:  gh.String(head),
		Base:  gh.String(base),
	})
	if err != nil {
		return 0, "", fmt.Errorf("create PR: %w", err)
	}
	return pr.GetNumber(), pr.GetHTMLURL(), nil
}

// CommentOnIssue posts a comment on an issue or pull request.
func (c *Client) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, number, &gh.IssueComment{
		Body: gh.String(body),
	})
	if err != nil {
		return fmt.Errorf("comment on issue: %w", err)
	}
	return nil
}

// MergePR merges a pull request with the given commit message.
func (c *Client) MergePR(ctx context.Context, owner, repo string, number int, message string) error {
	_, _, err := c.gh.PullRequests.Merge(ctx, owner, repo, number, message, nil)
	if err != nil {
		return fmt.Errorf("merge PR: %w", err)
	}
	return nil
}

// CreateReview submits a pull request review.
// event should be one of: "APPROVE", "REQUEST_CHANGES", "COMMENT".
func (c *Client) CreateReview(ctx context.Context, owner, repo string, prNumber int, body, event string) error {
	_, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, &gh.PullRequestReviewRequest{
		Body:  gh.String(body),
		Event: gh.String(event),
	})
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

// GetFileContent retrieves the text content of a file at a given ref (branch, tag, or SHA).
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	opts := &gh.RepositoryContentGetOptions{Ref: ref}
	file, _, _, err := c.gh.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", fmt.Errorf("get file content: %w", err)
	}
	if file == nil {
		return "", fmt.Errorf("get file content: path %q is a directory", path)
	}
	content, err := file.GetContent()
	if err != nil {
		return "", fmt.Errorf("decode file content: %w", err)
	}
	return content, nil
}
