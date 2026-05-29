package github

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v88/github"
)

func NewClient(_ context.Context) (*github.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	return github.NewClient(github.WithAuthToken(token))
}
