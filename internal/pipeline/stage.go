package pipeline

import "context"

// GitHubClient defines the GitHub operations needed by pipeline stages.
// This interface breaks the import cycle between pipeline and github packages.
type GitHubClient interface {
	CreatePR(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error)
	CreateReview(ctx context.Context, owner, repo string, prNumber int, body, event string) error
	MergePR(ctx context.Context, owner, repo string, number int, message string) error
	CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error
}

type StageStatus string

const (
	StatusPassed  StageStatus = "passed"
	StatusFailed  StageStatus = "failed"
	StatusSkipped StageStatus = "skipped"
)

type StageResult struct {
	Status  StageStatus
	Output  string
	Details map[string]any
}

type Stage interface {
	Name() string
	Run(ctx context.Context, state *PipelineState) (StageResult, error)
}
