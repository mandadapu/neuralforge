package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIssueLabeled(t *testing.T) {
	payload := []byte(`{
		"action": "labeled",
		"label": {"name": "neuralforge"},
		"issue": {
			"number": 42,
			"title": "Fix login bug",
			"body": "The login page crashes on submit",
			"user": {"login": "alice"},
			"labels": [{"name": "bug"}, {"name": "neuralforge"}]
		},
		"repository": {
			"full_name": "owner/repo",
			"default_branch": "main",
			"clone_url": "https://github.com/owner/repo.git"
		}
	}`)

	evt, err := ParseWebhookEvent("issues", payload)
	require.NoError(t, err)

	ie, ok := evt.(*IssueLabeledEvent)
	require.True(t, ok)
	assert.Equal(t, "neuralforge", ie.Label)
	assert.Equal(t, 42, ie.Issue.Number)
	assert.Equal(t, "owner/repo", ie.Repo.FullName)
}

func TestParseIssueComment(t *testing.T) {
	payload := []byte(`{
		"action": "created",
		"comment": {"body": "/retry", "user": {"login": "bob"}},
		"issue": {"number": 42, "title": "Fix it"},
		"repository": {"full_name": "owner/repo", "default_branch": "main", "clone_url": "https://github.com/owner/repo.git"}
	}`)

	evt, err := ParseWebhookEvent("issue_comment", payload)
	require.NoError(t, err)

	ce, ok := evt.(*IssueCommentEvent)
	require.True(t, ok)
	assert.Equal(t, "/retry", ce.Command)
	assert.Equal(t, 42, ce.Issue.Number)
}

func TestParseUnknownEvent(t *testing.T) {
	evt, err := ParseWebhookEvent("push", []byte(`{}`))
	require.NoError(t, err)
	assert.Nil(t, evt)
}
