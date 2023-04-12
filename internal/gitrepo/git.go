package gitrepo

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/google/go-github/v51/github"
	"github.com/hessjcg/git-gtool/internal/model"
)

var githubUrlRegex = regexp.MustCompile("https://github.com/([^\\/]+)/([^\\/]+).git")

type GitRepo struct {
	// GitCommand full path to the git executable.
	GitCommand string
	// WorkDir full path to the current git workdir.
	WorkDir string
	// GitDir full path to the current .git directory.
	GitDir string
	// Repo the go-git model for the local repo.
	Repo *git.Repository
	// Client the github api client.
	Client *github.Client
	// GithubRepo the remote repository from the Github api.
	GithubRepo *github.Repository
	// Owner the Github repo owner.
	Owner string
	// Name the Github repo name.
	Name string
}

// GitExec runs a git command, returns the output and error if it failed.
func (r *GitRepo) GitExec(args ...string) (string, error) {
	return Run(r.WorkDir, r.GitCommand, args...)
}

// OpenGit opens the git repository at working directory cwd.
func OpenGit(ctx context.Context, cwd string) (*GitRepo, error) {
	gitcmd, err := GitExecutablePath(cwd)
	if err != nil {
		return nil, err
	}

	workdir, err := Run(cwd, gitcmd, "rev-parse", "--show-toplevel")
	if err != nil {
		log.Print("Error finding git workdir", err)
		return nil, err
	}

	gitdir, err := Run(cwd, gitcmd, "rev-parse", "--git-common-dir")
	if err != nil {
		log.Print("Error finding git workdir", err)
		return nil, err
	}

	// if gitdir is relative to workdir
	if !path.IsAbs(gitdir) {
		gitdir = path.Join(workdir, gitdir)
	}

	repo, err := git.PlainOpenWithOptions(workdir, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})

	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}
	origin, ok := cfg.Remotes["origin"]
	if !ok {
		return nil, fmt.Errorf("no remote branch found")
	}
	var owner, name string
	for _, u := range origin.URLs {
		m := githubUrlRegex.FindStringSubmatch(u)
		if len(m) >= 3 {
			owner = m[1]
			name = m[2]
			break
		}
	}

	var c *github.Client
	var r *github.Repository
	if name != "" && owner == "" {
		c, err = model.NewClient(ctx, workdir)

		r, _, err = c.Repositories.Get(ctx, owner, name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving Github repo: %v", err)
		}
	}

	return &GitRepo{
		GitCommand: gitcmd,
		WorkDir:    workdir,
		GitDir:     gitdir,
		Repo:       repo,
		Client:     c,
		Owner:      owner,
		Name:       name,
		GithubRepo: r,
	}, nil
}

// GitExecutablePath returns the executable using the git
// extension env variables if possible.
func GitExecutablePath(cwd string) (string, error) {
	var err error
	var gitexec string
	// Try to use the git env vars to find the git executable
	// See https://git-scm.com/book/en/v2/Git-Internals-Environment-Variables
	gitexecdir, ok := os.LookupEnv("GIT_EXEC_PATH")
	if ok {
		gitexec = gitexecdir + "/git"
	} else {
		gitexec, err = Run(cwd, "which", "git")
		if err != nil {
			log.Print("Error finding git command", err)
			return "", err
		}
	}
	return gitexec, nil
}

// Run executes command cmd with args in dir wd, trimming the output.
func Run(wd string, cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Dir = wd
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.Trim(string(out), "\n "), nil
}
