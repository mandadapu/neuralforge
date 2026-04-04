package github

import (
	"encoding/json"
	"strings"

	"github.com/mandadapu/neuralforge/internal/pipeline"
)

// Event is the interface implemented by all parsed webhook events.
type Event interface {
	EventType() string
}

// IssueLabeledEvent is emitted when a label is added to an issue.
type IssueLabeledEvent struct {
	Label          string
	Issue          pipeline.GitHubIssue
	Repo           pipeline.RepoContext
	InstallationID int64
}

func (e *IssueLabeledEvent) EventType() string { return "issue_labeled" }

// IssueCommentEvent is emitted when a slash-command comment is created on an issue.
type IssueCommentEvent struct {
	Command        string
	User           string
	Issue          pipeline.GitHubIssue
	Repo           pipeline.RepoContext
	InstallationID int64
}

func (e *IssueCommentEvent) EventType() string { return "issue_comment" }

// ParseWebhookEvent parses a raw GitHub webhook payload into a typed Event.
// It returns (nil, nil) for unrecognized or irrelevant events.
func ParseWebhookEvent(eventType string, payload []byte) (Event, error) {
	switch eventType {
	case "issues":
		return parseIssueEvent(payload)
	case "issue_comment":
		return parseIssueCommentEvent(payload)
	default:
		return nil, nil
	}
}

// raw JSON structures for unmarshalling

type rawLabel struct {
	Name string `json:"name"`
}

type rawUser struct {
	Login string `json:"login"`
}

type rawIssue struct {
	Number int        `json:"number"`
	Title  string     `json:"title"`
	Body   string     `json:"body"`
	User   rawUser    `json:"user"`
	Labels []rawLabel `json:"labels"`
}

type rawRepo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

type rawComment struct {
	Body string  `json:"body"`
	User rawUser `json:"user"`
}

type rawInstallation struct {
	ID int64 `json:"id"`
}

type rawIssuePayload struct {
	Action       string          `json:"action"`
	Label        rawLabel        `json:"label"`
	Issue        rawIssue        `json:"issue"`
	Repo         rawRepo         `json:"repository"`
	Installation rawInstallation `json:"installation"`
}

type rawCommentPayload struct {
	Action       string          `json:"action"`
	Comment      rawComment      `json:"comment"`
	Issue        rawIssue        `json:"issue"`
	Repo         rawRepo         `json:"repository"`
	Installation rawInstallation `json:"installation"`
}

func parseIssueEvent(payload []byte) (Event, error) {
	var raw rawIssuePayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	if raw.Action != "labeled" {
		return nil, nil
	}

	labels := make([]string, len(raw.Issue.Labels))
	for i, l := range raw.Issue.Labels {
		labels[i] = l.Name
	}

	return &IssueLabeledEvent{
		Label: raw.Label.Name,
		Issue: pipeline.GitHubIssue{
			Number: raw.Issue.Number,
			Title:  raw.Issue.Title,
			Body:   raw.Issue.Body,
			Author: raw.Issue.User.Login,
			Labels: labels,
		},
		Repo: pipeline.RepoContext{
			FullName:      raw.Repo.FullName,
			DefaultBranch: raw.Repo.DefaultBranch,
			CloneURL:      raw.Repo.CloneURL,
		},
		InstallationID: raw.Installation.ID,
	}, nil
}

func parseIssueCommentEvent(payload []byte) (Event, error) {
	var raw rawCommentPayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	if raw.Action != "created" {
		return nil, nil
	}

	body := strings.TrimSpace(raw.Comment.Body)
	if !strings.HasPrefix(body, "/") {
		return nil, nil
	}

	labels := make([]string, len(raw.Issue.Labels))
	for i, l := range raw.Issue.Labels {
		labels[i] = l.Name
	}

	return &IssueCommentEvent{
		Command: body,
		User:    raw.Comment.User.Login,
		Issue: pipeline.GitHubIssue{
			Number: raw.Issue.Number,
			Title:  raw.Issue.Title,
			Body:   raw.Issue.Body,
			Author: raw.Issue.User.Login,
			Labels: labels,
		},
		Repo: pipeline.RepoContext{
			FullName:      raw.Repo.FullName,
			DefaultBranch: raw.Repo.DefaultBranch,
			CloneURL:      raw.Repo.CloneURL,
		},
		InstallationID: raw.Installation.ID,
	}, nil
}
