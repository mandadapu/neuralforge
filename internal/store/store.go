package store

import "time"

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID            string     `json:"id"`
	RepoFullName  string     `json:"repo_full_name"`
	IssueNumber   int        `json:"issue_number"`
	IssueTitle    string     `json:"issue_title"`
	Status        JobStatus  `json:"status"`
	CurrentStage  string     `json:"current_stage"`
	PipelineState string     `json:"pipeline_state"`
	Error         string     `json:"error"`
	CostUSD       float64    `json:"cost_usd"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

type RepoContextRecord struct {
	RepoFullName   string    `json:"repo_full_name"`
	ClaudeMDHash   string    `json:"claude_md_hash"`
	LastAnalyzedAt time.Time `json:"last_analyzed_at"`
	FileCount      int       `json:"file_count"`
	Languages      []string  `json:"languages"`
}

type Store interface {
	CreateJob(job Job) error
	GetJob(id string) (*Job, error)
	GetJobByIssue(repoFullName string, issueNumber int) (*Job, error)
	UpdateJobStatus(id string, status JobStatus, stage string) error
	UpdateJobError(id string, errMsg string) error
	UpdateJobCost(id string, cost float64) error
	CompleteJob(id string, status JobStatus) error
	ListPendingJobs(limit int) ([]Job, error)
	UpsertRepoContext(ctx RepoContextRecord) error
	GetRepoContext(repoFullName string) (*RepoContextRecord, error)
	Migrate() error
	Close() error
}
