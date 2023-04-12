package gitrepo

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Run(wd string, cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Dir = wd
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.Trim(string(out), "\n "), nil
}

func (c *GitRepo) IsGHToolInstalled() bool {
	// warning, not concurrency safe.
	if c.ghInstalled == nil {
		c.ghInstalled = new(bool)
		_, err := Run(c.WorkDir, "gh", "auth", "status")
		if err != nil {
			*c.ghInstalled = false
		} else {
			*c.ghInstalled = true
		}
	}
	return *c.ghInstalled
}

type GHPullRequest struct {
	Assignees []struct {
		Id    string `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"assignees"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Id          string `json:"id"`
	MergeCommit struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	MergedAt time.Time `json:"mergedAt"`
	MergedBy struct {
		Login string `json:"login"`
	} `json:"mergedBy"`
	Number      int    `json:"number"`
	State       string `json:"state"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	IsDraft     bool   `json:"isDraft"`
}

const prFields = "id,number,title,author,assignees,mergeCommit,mergedAt,mergedBy,state,headRefName,baseRefName,isDraft"

func (c *GitRepo) ListPullRequests(prNums []int) ([]GHPullRequest, error) {
	args := []string{"pr", "list", "--state", "all",
		"--json", prFields}

	if len(prNums) > 0 {
		s := make([]string, len(prNums))
		for i, num := range prNums {
			s[i] = strconv.Itoa(num)
		}
		prNumStr := strings.Join(s, " ")
		args = append(args, "--search", prNumStr)
	}

	out, err := Run(c.WorkDir, "gh", args...)
	if err != nil {
		return nil, err
	}
	var prs []GHPullRequest
	err = json.Unmarshal([]byte(out), &prs)
	if err != nil {
		return nil, err
	}
	return prs, nil
}

func (c *GitRepo) ListOpenPullRequests() ([]GHPullRequest, error) {
	args := []string{"pr", "list", "--state", "OPEN",
		"--json", prFields}

	out, err := Run(c.WorkDir, "gh", args...)
	if err != nil {
		return nil, err
	}
	var prs []GHPullRequest
	err = json.Unmarshal([]byte(out), &prs)
	if err != nil {
		return nil, err
	}
	return prs, nil
}
