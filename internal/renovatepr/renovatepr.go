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

package renovatepr

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v51/github"
	"github.com/hessjcg/git-gtool/internal/gitrepo"
	"github.com/hessjcg/git-gtool/internal/model"
)

var (
	approve         = "APPROVE"
	lgtm            = "LGTM"
	ErrFailedCheck  = fmt.Errorf("check failed")
	ErrMissingCheck = fmt.Errorf("check missing")
)

// MergePRs finds all open PRs submitted by `renovate-bot` and attempts
// to merge them.
func MergePRs(ctx context.Context, repo *gitrepo.GitRepo) error {
	var err error
	var hasMore bool
	errCount := 0
	for i := 1; i < 100 && errCount < 10; i++ {
		log.Printf("Merge Renovate PRs iteration %v", i)
		hasMore, err = mergeStep(ctx, repo)
		if !hasMore {
			log.Printf("No more work to do")
			break
		}
		if err != nil {
			errCount++
			log.Printf("Error: %v", err)
			log.Printf("Sleeping for 2 minutes before trying again...")
			time.Sleep(2 * time.Minute)
		} else {
			log.Printf("Successfully merged PR. Attempting to merge another...")
			errCount = 0
		}
	}
	return err
}

// mergeStep Do one iteration, attempting to merge the oldest renovate-bot PR.
// returns true when the command should attempt another step, and error if there
// was an error during this step.
func mergeStep(ctx context.Context, r *gitrepo.GitRepo) (bool, error) {

	log.Printf("Listing renovate PRs for %v/%v targeting branch %v", r.Owner, r.Name, r.GithubRepo.GetDefaultBranch())

	// list all open PRs in order
	g := &model.ListGenerator[github.PullRequest]{
		Retrieve: func(opts github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
			return r.Client.PullRequests.List(ctx, r.Owner, r.Name, &github.PullRequestListOptions{
				Sort:        "created",
				State:       "open",
				Base:        r.GithubRepo.GetDefaultBranch(),
				ListOptions: opts,
			})
		},
	}

	// filter all open PRs to just Renovate PRs
	renovatePrs := make([]*github.PullRequest, 0, 20)
	for g.HasNext() {
		pr, err := g.Next()
		if err != nil {
			return false, err
		}
		if pr.GetUser().GetLogin() == "renovate-bot" {
			renovatePrs = append(renovatePrs, pr)
		}
	}

	if len(renovatePrs) == 0 {
		log.Printf("No open Renovate PRs.")
		return false, nil
	}

	// Determine the Active PR
	// Use the first Mergable PR and if none found, then use the oldest PR
	activePr := chooseActivePr(renovatePrs)

	// Approve pending workflow runs
	err := approveWorkflowRuns(ctx, r.Client, r.Owner, r.Name, activePr)
	if err != nil {
		return true, err
	}

	// Check Statuses Pass
	err = checkStatusChecks(ctx, r.Client, r.Owner, r.Name, r.GithubRepo.GetDefaultBranch(), activePr)
	if err == ErrMissingCheck {
		return true, err
	}
	if err != nil {
		return false, err
	}

	// Approve the PR
	err = approvePr(ctx, r.Client, r.Owner, r.Name, activePr)
	if err != nil {
		return true, err
	}

	return true, mergePr(ctx, r.Client, r.Owner, r.Name, activePr)
}

func checkStatusChecks(ctx context.Context, client *github.Client, org string, repo string, base string, activePr *github.PullRequest) error {
	// Holds combined check results from both status checks and workflow check runs.
	checkResults := map[string]string{}

	// List required status checks for the repo
	requiredChecks, _, err := client.Repositories.GetRequiredStatusChecks(ctx, org, repo, base)
	if err != nil {
		return err
	}
	for _, c := range requiredChecks.Checks {
		context := c.Context
		if c.AppID != nil {
			context = fmt.Sprintf("%s/%d", c.Context, *c.AppID)
		}
		// Set the status check to "missing" by default
		checkResults[context] = "missing"
	}

	var count int
	// Load statuses and update checkResults
	statuses, err := checkStatuses(ctx, client, org, repo, activePr, count)
	if err != nil {
		return err
	}
	for _, check := range statuses {
		checkResults[check.context] = check.conclusion
		if _, ok := checkResults[check.context]; ok {
			checkResults[check.context] = check.conclusion
		}
	}

	// load workload check runs and update checkResults
	checks, err := checkCheckRuns(ctx, client, org, repo, activePr)
	if err != nil {
		return err
	}
	for _, check := range checks {
		context := check.context
		if check.appId != nil {
			context = fmt.Sprintf("%s/%d", check.context, *check.appId)
		}
		if _, ok := checkResults[context]; ok {
			checkResults[context] = check.conclusion
		}
	}
	var failedCheck bool
	var missingCheck bool
	for context, conclusion := range checkResults {
		log.Printf("  required check %v: %v", context, conclusion)
		switch conclusion {
		case "success":
			continue // do nothing
		case "failed":
			failedCheck = true
		default:
			missingCheck = true
		}
	}
	if failedCheck {
		return ErrFailedCheck
	}
	if missingCheck || len(checkResults) == 0 {
		return ErrMissingCheck
	}
	return nil
}

// approvePr checks if there is not yet an "approve" review, and adds one.
func approvePr(ctx context.Context, client *github.Client, org string, repo string, activePr *github.PullRequest) error {
	// Check if the PR has been approved
	rg := &model.ListGenerator[github.PullRequestReview]{
		Retrieve: func(opts github.ListOptions) ([]*github.PullRequestReview, *github.Response, error) {
			return client.PullRequests.ListReviews(ctx, org, repo, activePr.GetNumber(), &opts)
		},
	}

	var approved bool
	for rg.HasNext() {
		review, err := rg.Next()
		if err != nil {
			return fmt.Errorf("Can't get review: %v", err)
		}
		if review.GetState() == "APPROVED" {
			approved = true
		}
	}
	if approved {
		return nil
	}

	// Attempt to approve the PR
	log.Printf("Approving PR #%4d with LGTM message.", activePr.GetNumber())
	lgtmReview, _, err := client.PullRequests.CreateReview(ctx, org, repo, activePr.GetNumber(), &github.PullRequestReviewRequest{
		NodeID:   activePr.NodeID,
		Body:     &lgtm,
		CommitID: activePr.Head.SHA,
		Event:    &approve,
	})
	if err != nil {
		return fmt.Errorf("can't create LGTM review: %v/%v %v %v %v", org, repo, activePr.GetNumber(), activePr.GetTitle(), err)
	}
	client.PullRequests.SubmitReview(ctx, org, repo, activePr.GetNumber(), lgtmReview.GetID(), &github.PullRequestReviewRequest{
		NodeID:   activePr.NodeID,
		CommitID: activePr.Head.SHA,
		Body:     &lgtm,
		Event:    &approve,
	})
	if err != nil {
		return fmt.Errorf("can't submit LGTM review: %v/%v %v %v %v", org, repo, activePr.GetNumber(), activePr.GetTitle(), err)
	}
	return nil
}

// chooseActivePr returns the oldest PR that is mergeable or nil if none exists.
func chooseActivePr(renovatePrs []*github.PullRequest) *github.PullRequest {
	var activePr *github.PullRequest
	for _, pr := range renovatePrs {
		log.Printf("#%4d %s %s", pr.GetNumber(), pr.GetUser().GetLogin(), pr.GetTitle())
		if pr.GetMergeable() {
			activePr = pr
		}
	}
	if activePr == nil {
		activePr = renovatePrs[0]
		log.Println()
		log.Printf("Attempting to merge PR:")
		log.Printf("#%d %v", activePr.GetNumber(), activePr.GetTitle())
	}

	return activePr
}

type runResult struct {
	appId      *int64
	context    string
	conclusion string
}

// checkStatuses returns a list of github statuses as run results.
func checkStatuses(ctx context.Context, client *github.Client, org string, repo string, activePr *github.PullRequest, count int) ([]runResult, error) {
	var results []runResult

	// Load statuses from github api
	reqStatusG := &model.PagedListGenerator[github.CombinedStatus, github.RepoStatus]{
		Retrieve: func(opts github.ListOptions) (*github.CombinedStatus, []*github.RepoStatus, *github.Response, error) {
			pg, res, err := client.Repositories.GetCombinedStatus(ctx, org, repo, activePr.Head.GetSHA(), &opts)
			if err != nil {
				return nil, nil, res, err
			}
			return pg, pg.Statuses, res, err
		},
	}
	for reqStatusG.HasNext() {
		_, status, err := reqStatusG.Next()
		if err != nil {
			return nil, fmt.Errorf("can't list workflows: %v/%v %v %v %v", org, repo, activePr.GetNumber(), activePr.GetTitle(), err)
		}
		results = append(results, runResult{
			context:    status.GetContext(),
			conclusion: status.GetState(),
		})
	}
	return results, nil
}

// approveWorkflowRuns determines if there are workflow runs for the current PR
// head commit that are pending approval from a repository owner, and submits
// approval to start the workflow runs.
func approveWorkflowRuns(ctx context.Context, client *github.Client, org string, repo string, activePr *github.PullRequest) error {
	wfg := &model.PagedListGenerator[github.WorkflowRuns, github.WorkflowRun]{
		Retrieve: func(opts github.ListOptions) (*github.WorkflowRuns, []*github.WorkflowRun, *github.Response, error) {
			r, req, err := client.Actions.ListRepositoryWorkflowRuns(ctx, org, repo, &github.ListWorkflowRunsOptions{
				Event:       "pull_request",
				Status:      "action_required",
				Branch:      activePr.Head.GetRef(),
				ListOptions: opts,
			})
			if err != nil {
				return nil, nil, req, err
			}
			return r, r.WorkflowRuns, req, err
		},
	}

	for wfg.HasNext() {
		_, r, err := wfg.Next()
		if err != nil {
			return err
		}
		if r.GetHeadSHA() != activePr.GetHead().GetSHA() {
			continue
		}

		log.Printf(" Approving run: %v %v %v", r.GetURL(), r.GetConclusion(), r.GetHeadBranch())
		req, err := client.NewRequest("POST", r.GetURL()+"/approve", nil)
		if err != nil {
			return err
		}
		_, err = client.Do(ctx, req, nil)
		if err != nil {
			return fmt.Errorf("Can't approve workflow: %v", err)
		}
	}
	return nil
}

// checkCheckRuns returns a list of run results for workflow check runs, which
// confusingly is a different API from statuses.
func checkCheckRuns(ctx context.Context, client *github.Client, org string, repo string, activePr *github.PullRequest) ([]runResult, error) {
	var results []runResult

	// Checks
	reqStatusG := &model.PagedListGenerator[github.ListCheckRunsResults, github.CheckRun]{
		Retrieve: func(opts github.ListOptions) (*github.ListCheckRunsResults, []*github.CheckRun, *github.Response, error) {
			pg, res, err := client.Checks.ListCheckRunsForRef(ctx, org, repo, activePr.Head.GetSHA(), &github.ListCheckRunsOptions{
				ListOptions: opts,
			})
			if err != nil {
				return nil, nil, res, err
			}
			return pg, pg.CheckRuns, res, err
		},
	}
	for reqStatusG.HasNext() {
		_, status, err := reqStatusG.Next()
		if err != nil {
			return nil, fmt.Errorf("can't list check runs: %v/%v %v %v %v", org, repo, activePr.GetNumber(), activePr.GetTitle(), err)
		}
		conclusion := status.GetConclusion()
		if conclusion == "" {
			conclusion = status.GetStatus()
		}
		results = append(results, runResult{
			appId:      status.GetApp().ID,
			context:    status.GetName(),
			conclusion: conclusion,
		})
	}
	return results, nil
}

// mergePr attempts to do a rebase+squash of this PR onto the default branch.
func mergePr(ctx context.Context, client *github.Client, org, repo string, activePr *github.PullRequest) error {
	log.Printf("Attempting to merge #%4d %s ", activePr.GetNumber(), activePr.GetTitle())
	activePr, _, err := client.PullRequests.Get(ctx, org, repo, activePr.GetNumber())
	if err != nil {
		return err
	}

	// When the PR is mergable, attempt to merge it
	if !activePr.GetRebaseable() {
		return fmt.Errorf("unable to merge %v via squash method, it is not rebaseable", activePr.GetNumber())
	}
	mergeResult, _, err := client.PullRequests.Merge(ctx, org, repo, activePr.GetNumber(), activePr.GetState(), &github.PullRequestOptions{
		MergeMethod: "squash",
		CommitTitle: activePr.GetTitle(),
	})
	if mergeResult != nil {
		log.Printf("  merged: %v, %s", mergeResult.GetMerged(), mergeResult.GetMessage())
		if mergeResult.GetMerged() {
			return nil
		}
		return fmt.Errorf("unable to merge %v via squash method: %v", activePr.GetNumber(), mergeResult.GetMessage())
	}
	if err != nil {
		return fmt.Errorf("unable to merge %v via squash method: %v", activePr.GetNumber(), err)
	}

	return nil
}
