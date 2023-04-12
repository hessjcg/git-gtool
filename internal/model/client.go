package model

import (
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/google/go-github/v51/github"
	"golang.org/x/oauth2"
)

func NewClient(ctx context.Context, cwd string) (*github.Client, error) {
	cmd := exec.Command("gh", "auth", "token")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Unable to get github token using gh")
	}
	token := strings.Trim(string(output), "\n\r ")
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return client, nil
}
