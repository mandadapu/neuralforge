package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyStageEmptyCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"empty string defaults to make test", "", "make test"},
		{"whitespace only defaults to make test", "   ", "make test"},
		{"tab and spaces defaults to make test", " \t ", "make test"},
		{"valid command preserved", "go test ./...", "go test ./..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := NewVerifyStage(tt.command)
			assert.Equal(t, tt.want, stage.command)
		})
	}
}

func TestVerifyStageRunEmptyParts(t *testing.T) {
	// Directly construct VerifyStage with empty command (bypassing constructor)
	// to verify Run() doesn't panic
	s := &VerifyStage{command: ""}
	state := &PipelineState{Repo: RepoContext{LocalPath: "/tmp"}}
	result, err := s.Run(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "no verify command configured", result.Output)
}

func TestVerifyStageRunWhitespaceOnlyParts(t *testing.T) {
	// Directly construct with whitespace-only command (bypassing constructor)
	s := &VerifyStage{command: "   "}
	state := &PipelineState{Repo: RepoContext{LocalPath: "/tmp"}}
	result, err := s.Run(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "no verify command configured", result.Output)
}

func TestVerifyStageRunNoLocalPath(t *testing.T) {
	s := NewVerifyStage("echo hello")
	state := &PipelineState{}
	result, err := s.Run(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "no local path available", result.Output)
}

func TestVerifyStageRunValidCommand(t *testing.T) {
	s := NewVerifyStage("echo hello")
	state := &PipelineState{Repo: RepoContext{LocalPath: "/tmp"}}
	result, err := s.Run(context.Background(), state)
	assert.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.NotNil(t, state.TestResults)
	assert.True(t, state.TestResults.Passed)
}

func TestVerifyStageName(t *testing.T) {
	s := NewVerifyStage("")
	assert.Equal(t, "verify", s.Name())
}
