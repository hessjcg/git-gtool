// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/google/go-github/v51/github"
	"golang.org/x/oauth2"
)

// NewClient returns a new Github client that uses the same credentials
// as the `gh` Github command line client.
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
