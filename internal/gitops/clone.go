package gitops

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type CloneResult struct {
	Dir     string
	Cleanup func()
}

func CloneRepo(repoURL, branch, token string) (*CloneResult, error) {
	dir, err := os.MkdirTemp("", "driftwatch-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	opts := &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	}

	if token != "" {
		opts.Auth = &http.BasicAuth{
			Username: "x-token",
			Password: token,
		}
	}

	_, err = git.PlainClone(dir, false, opts)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("clone %s: %w", repoURL, err)
	}

	return &CloneResult{Dir: dir, Cleanup: cleanup}, nil
}
