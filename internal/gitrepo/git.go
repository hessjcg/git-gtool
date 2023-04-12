package gitrepo

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v51/github"
	"github.com/hessjcg/git-gtool/internal/model"
)

var githubUrlRegex = regexp.MustCompile("https://github.com/([^\\/]+)/([^\\/]+).git")

type GitRepo struct {
	GitCommand  string
	WorkDir     string
	GitDir      string
	Repo        *git.Repository
	Client      *github.Client
	ghInstalled *bool
	GithubRepo  *github.Repository
	// Owner the github owner
	Owner string
	// NAme the Github repo name
	Name string
}

type CommitRange struct {
	Commits  []*object.Commit
	Base     *object.Commit
	Target   *object.Commit
	Ancestor *object.Commit
	Linear   bool
}

func (r *GitRepo) GitExec(args ...string) (string, error) {
	return Run(r.WorkDir, r.GitCommand, args...)
}

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

func GitExecutablePath(cwd string) (string, error) {
	var err error
	var gitexec string
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
