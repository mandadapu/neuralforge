package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Git struct {
	dir string
}

func New(dir string) *Git {
	return &Git{dir: dir}
}

func Clone(url, dir, token string) (*Git, error) {
	cloneURL := url
	if token != "" {
		cloneURL = strings.Replace(url, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}
	if err := runCmd("git", "clone", "--depth=1", cloneURL, dir); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	return New(dir), nil
}

func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", args[0], stderr.String(), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g *Git) CreateBranch(name string) error {
	_, err := g.run("checkout", "-b", name)
	return err
}

func (g *Git) Checkout(name string) error {
	_, err := g.run("checkout", name)
	return err
}

func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (g *Git) AddAll() error {
	_, err := g.run("add", "-A")
	return err
}

func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

func (g *Git) Push(remote, branch string) error {
	_, err := g.run("push", remote, branch)
	return err
}

func (g *Git) PushNewBranch(remote, branch string) error {
	_, err := g.run("push", "-u", remote, branch)
	return err
}

func (g *Git) Log(n int) (string, error) {
	return g.run("log", fmt.Sprintf("-%d", n), "--oneline")
}

func (g *Git) DiffStat(base string) (string, error) {
	return g.run("diff", "--stat", base)
}

func (g *Git) DiffLines(base string) (int, error) {
	out, err := g.run("diff", "--shortstat", base)
	if err != nil {
		return 0, err
	}
	lines := 0
	for _, part := range strings.Split(out, ",") {
		part = strings.TrimSpace(part)
		var n int
		if strings.Contains(part, "insertion") {
			fmt.Sscanf(part, "%d", &n)
			lines += n
		} else if strings.Contains(part, "deletion") {
			fmt.Sscanf(part, "%d", &n)
			lines += n
		}
	}
	return lines, nil
}

func (g *Git) FilesChanged(base string) ([]string, error) {
	out, err := g.run("diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (g *Git) Dir() string {
	return g.dir
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %s: %w", name, args[0], stderr.String(), err)
	}
	return nil
}
