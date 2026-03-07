package store

import (
	"context"
	"time"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID             string     `json:"id"`
	RepoFullName   string     `json:"repo_full_name"`
	IssueNumber    int        `json:"issue_number"`
	IssueTitle     string     `json:"issue_title"`
	InstallationID int64      `json:"installation_id"`
	Status         JobStatus  `json:"status"`
	CurrentStage   string     `json:"current_stage"`
	PipelineState  string     `json:"pipeline_state"`
	Error          string     `json:"error"`
	CostUSD        float64    `json:"cost_usd"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type RepoContextRecord struct {
	RepoFullName   string    `json:"repo_full_name"`
	ClaudeMDHash   string    `json:"claude_md_hash"`
	LastAnalyzedAt time.Time `json:"last_analyzed_at"`
	FileCount      int       `json:"file_count"`
	Languages      []string  `json:"languages"`
}

type Store interface {
	CreateJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context, id string) (*Job, error)
	GetJobByIssue(ctx context.Context, repoFullName string, issueNumber int) (*Job, error)
	UpdateJobStatus(ctx context.Context, id string, status JobStatus, stage string) error
	UpdateJobError(ctx context.Context, id string, errMsg string) error
	UpdateJobCost(ctx context.Context, id string, cost float64) error
	CompleteJob(ctx context.Context, id string, status JobStatus) error
	ListPendingJobs(ctx context.Context, limit int) ([]Job, error)
	UpsertRepoContext(ctx context.Context, rc RepoContextRecord) error
	GetRepoContext(ctx context.Context, repoFullName string) (*RepoContextRecord, error)
	Migrate(ctx context.Context) error
	Close() error
}
