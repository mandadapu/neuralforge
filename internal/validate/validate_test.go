package validate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoFullName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "owner/repo", false},
		{"valid with special chars", "my-org/my.repo_1", false},
		{"valid with dots", "org.name/repo.name", false},
		{"empty string", "", true},
		{"no slash", "noslash", true},
		{"too many slashes", "a/b/c", true},
		{"shell injection", "owner/repo; rm -rf /", true},
		{"command substitution", "$(evil)/repo", true},
		{"backtick injection", "`evil`/repo", true},
		{"space in name", "owner/ repo", true},
		{"newline in name", "owner/repo\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RepoFullName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid neuralforge branch", "neuralforge/issue-42", false},
		{"valid main", "main", false},
		{"valid feature branch", "feat/x.y", false},
		{"valid with underscore", "feature_branch", false},
		{"empty string", "", true},
		{"command substitution", "$(cmd)", true},
		{"backtick injection", "`cmd`", true},
		{"double dot", "a..b", true},
		{"space in name", "branch name", true},
		{"semicolon injection", "; rm -rf /", true},
		{"pipe injection", "main|evil", true},
		{"too long", strings.Repeat("a", 251), true},
		{"single char", "a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BranchName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJobID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid job ID", "owner/repo#42", false},
		{"valid with special chars", "my-org/my.repo_1#123", false},
		{"no hash", "nohash", true},
		{"invalid repo part", "bad name#1", true},
		{"missing number part", "owner/repo", true},
		{"shell injection in repo", "$(evil)/repo#1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := JobID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
