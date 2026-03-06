package validate

import (
	"fmt"
	"regexp"
	"strings"
)

// repoNameRe matches "owner/repo" where both parts are GitHub-valid:
// alphanumeric, hyphens, dots, underscores; 1-100 chars each.
var repoNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,100}/[a-zA-Z0-9._-]{1,100}$`)

// branchRe matches safe git branch names: alphanumeric, hyphens, underscores,
// dots, slashes — no shell metacharacters, no "..", no leading/trailing dots.
var branchRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]{0,248}[a-zA-Z0-9]$`)

// RepoFullName validates that s is a valid "owner/repo" GitHub identifier.
func RepoFullName(s string) error {
	if !repoNameRe.MatchString(s) {
		return fmt.Errorf("invalid repo full name %q: must match owner/repo", s)
	}
	return nil
}

// BranchName validates that s is safe for use as a git branch name.
// Rejects shell metacharacters, "..", and whitespace.
func BranchName(s string) error {
	if len(s) < 1 || len(s) > 250 {
		return fmt.Errorf("invalid branch name %q: length must be 1-250", s)
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("invalid branch name %q: contains '..'", s)
	}
	if !branchRe.MatchString(s) {
		return fmt.Errorf("invalid branch name %q: contains unsafe characters", s)
	}
	return nil
}

// JobID validates that s is a well-formed job ID (owner/repo#number).
func JobID(s string) error {
	parts := strings.SplitN(s, "#", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid job ID %q: must contain '#'", s)
	}
	if err := RepoFullName(parts[0]); err != nil {
		return fmt.Errorf("invalid job ID %q: %w", s, err)
	}
	return nil
}
