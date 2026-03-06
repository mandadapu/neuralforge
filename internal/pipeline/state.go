package pipeline

import "time"

type GitHubIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
	Author string   `json:"author"`
}

type RepoContext struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	LocalPath     string `json:"local_path"`
}

type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Diff   string `json:"diff"`
}

type TestReport struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Command string `json:"command"`
}

type ComplianceReport struct {
	Passed       bool     `json:"passed"`
	DiffLines    int      `json:"diff_lines"`
	FilesChanged int      `json:"files_changed"`
	Violations   []string `json:"violations"`
}

type StageLog struct {
	Name      string        `json:"name"`
	Status    StageStatus   `json:"status"`
	Output    string        `json:"output"`
	Duration  time.Duration `json:"duration"`
	StartedAt time.Time     `json:"started_at"`
}

type PipelineState struct {
	JobID         string            `json:"job_id"`
	Issue         GitHubIssue       `json:"issue"`
	Repo          RepoContext       `json:"repo"`
	Memory        string            `json:"memory"`
	Plan          string            `json:"plan"`
	SecurityNotes string            `json:"security_notes"`
	Changes       []FileChange      `json:"changes"`
	TestResults   *TestReport       `json:"test_results"`
	Compliance    *ComplianceReport `json:"compliance"`
	PRURL         string            `json:"pr_url"`
	PRNumber      int               `json:"pr_number"`
	ReviewNotes   string            `json:"review_notes"`
	Merged        bool              `json:"merged"`
	DeployURL     string            `json:"deploy_url"`
	StartedAt     time.Time         `json:"started_at"`
	Cost          float64           `json:"cost"`
	Stages        []StageLog        `json:"stages"`
}
