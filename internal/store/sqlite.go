package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDBTimeout is the maximum time allowed for a single database operation.
const DefaultDBTimeout = 5 * time.Second

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS jobs (
			id              TEXT PRIMARY KEY,
			repo_full_name  TEXT NOT NULL,
			issue_number    INTEGER NOT NULL,
			issue_title     TEXT NOT NULL DEFAULT '',
			installation_id INTEGER NOT NULL DEFAULT 0,
			status          TEXT NOT NULL DEFAULT 'queued',
			current_stage   TEXT NOT NULL DEFAULT '',
			pipeline_state  TEXT NOT NULL DEFAULT '',
			error           TEXT NOT NULL DEFAULT '',
			cost_usd        REAL NOT NULL DEFAULT 0,
			created_at      DATETIME NOT NULL,
			updated_at      DATETIME NOT NULL,
			completed_at    DATETIME
		);

		CREATE TABLE IF NOT EXISTS repo_contexts (
			repo_full_name   TEXT PRIMARY KEY,
			claude_md_hash   TEXT NOT NULL DEFAULT '',
			last_analyzed_at DATETIME NOT NULL,
			file_count       INTEGER NOT NULL DEFAULT 0,
			languages        TEXT NOT NULL DEFAULT '[]'
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Add installation_id column for databases created before this migration.
	_, err = s.db.ExecContext(ctx,
		`ALTER TABLE jobs ADD COLUMN installation_id INTEGER NOT NULL DEFAULT 0`)
	if err != nil {
		// Ignore "duplicate column" error — column already exists.
		if !isDuplicateColumnErr(err) {
			return fmt.Errorf("migrate add installation_id: %w", err)
		}
	}

	return nil
}

func isDuplicateColumnErr(err error) bool {
	return strings.Contains(err.Error(), "duplicate column")
}

func (s *SQLiteStore) CreateJob(ctx context.Context, job Job) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (id, repo_full_name, issue_number, issue_title, installation_id, status, current_stage, pipeline_state, error, cost_usd, created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.RepoFullName, job.IssueNumber, job.IssueTitle,
		job.InstallationID, job.Status, job.CurrentStage, job.PipelineState,
		job.Error, job.CostUSD, now, now, nil,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (s *SQLiteStore) scanJob(row interface{ Scan(...any) error }) (*Job, error) {
	var j Job
	var completedAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.RepoFullName, &j.IssueNumber, &j.IssueTitle,
		&j.InstallationID, &j.Status, &j.CurrentStage, &j.PipelineState,
		&j.Error, &j.CostUSD, &j.CreatedAt, &j.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}

func (s *SQLiteStore) GetJob(ctx context.Context, id string) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repo_full_name, issue_number, issue_title, installation_id, status, current_stage, pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE id = ?`, id,
	)
	j, err := s.scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

func (s *SQLiteStore) GetJobByIssue(ctx context.Context, repoFullName string, issueNumber int) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	row := s.db.QueryRowContext(ctx,
		`SELECT id, repo_full_name, issue_number, issue_title, installation_id, status, current_stage, pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE repo_full_name = ? AND issue_number = ?`, repoFullName, issueNumber,
	)
	j, err := s.scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("get job by issue: %w", err)
	}
	return j, nil
}

func (s *SQLiteStore) UpdateJobStatus(ctx context.Context, id string, status JobStatus, stage string) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, current_stage = ?, updated_at = ? WHERE id = ?`,
		status, stage, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateJobError(ctx context.Context, id string, errMsg string) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET error = ?, status = ?, updated_at = ? WHERE id = ?`,
		errMsg, JobFailed, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update job error: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateJobCost(ctx context.Context, id string, cost float64) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET cost_usd = ?, updated_at = ? WHERE id = ?`,
		cost, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update job cost: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CompleteJob(ctx context.Context, id string, status JobStatus) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id,
	)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListPendingJobs(ctx context.Context, limit int) ([]Job, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, repo_full_name, issue_number, issue_title, installation_id, status, current_stage, pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE status = ? ORDER BY created_at ASC LIMIT ?`,
		JobQueued, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		j, err := s.scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pending job: %w", err)
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) UpsertRepoContext(ctx context.Context, rc RepoContextRecord) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	langs, err := json.Marshal(rc.Languages)
	if err != nil {
		return fmt.Errorf("marshal languages: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO repo_contexts (repo_full_name, claude_md_hash, last_analyzed_at, file_count, languages)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(repo_full_name) DO UPDATE SET
			claude_md_hash = excluded.claude_md_hash,
			last_analyzed_at = excluded.last_analyzed_at,
			file_count = excluded.file_count,
			languages = excluded.languages`,
		rc.RepoFullName, rc.ClaudeMDHash, rc.LastAnalyzedAt.UTC(), rc.FileCount, string(langs),
	)
	if err != nil {
		return fmt.Errorf("upsert repo context: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetRepoContext(ctx context.Context, repoFullName string) (*RepoContextRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultDBTimeout)
	defer cancel()
	var rc RepoContextRecord
	var langsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT repo_full_name, claude_md_hash, last_analyzed_at, file_count, languages
		 FROM repo_contexts WHERE repo_full_name = ?`, repoFullName,
	).Scan(&rc.RepoFullName, &rc.ClaudeMDHash, &rc.LastAnalyzedAt, &rc.FileCount, &langsJSON)
	if err != nil {
		return nil, fmt.Errorf("get repo context: %w", err)
	}
	if err := json.Unmarshal([]byte(langsJSON), &rc.Languages); err != nil {
		return nil, fmt.Errorf("unmarshal languages: %w", err)
	}
	return &rc, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
