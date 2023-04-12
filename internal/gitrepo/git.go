package gitrepo

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func (r *GitRepo) GetCommit(commitIsh string) (*object.Commit, error) {
	hash, err := r.GitExec("rev-parse", commitIsh)
	if err != nil {
		return nil, err
	}
	return r.Repo.CommitObject(plumbing.NewHash(hash))
}

func (r *GitRepo) ApplyPatch(patch *object.Patch, stackName string) error {
	patchFilename := path.Join(r.GitDir, "stack", "0000-rewrite.patch")
	f, err := os.Create(patchFilename)
	if err != nil {
		return fmt.Errorf("can't write patch file, %v", err)
	}
	patch.Encode(f)
	log.Print("Patching commit...")
	out, err := r.GitExec("apply", "--index", "--3way", "--allow-empty", "-v", patchFilename)
	log.Print(out)
	if err != nil {
		log.Print("Patch failed to apply. ")
		log.Print("Manually apply patch hunks in **/*.rej files. Then use `git add` to")
		log.Print("add the files to the index. Finally, run this command to continue: ")
		log.Printf("  git stack rewrite --stack %s --continue", stackName)
		return fmt.Errorf("patch did not apply clean, %v", err)
	}
	log.Print("Patch succeeded.")
	return nil
}

func (r *GitRepo) NewBranch(branch string, hash plumbing.Hash) (*plumbing.Reference, error) {
	n := plumbing.NewBranchReferenceName(branch)
	ref := plumbing.NewHashReference(n, hash)

	// The created reference is saved in the storage.
	err := r.Repo.Storer.SetReference(ref)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func (r *GitRepo) ListCommits(target plumbing.Hash, base plumbing.Hash) (CommitRange, error) {
	tc, err := r.Repo.CommitObject(target)
	if err != nil {
		return CommitRange{}, err
	}

	bc, err := r.Repo.CommitObject(base)
	if err != nil {
		return CommitRange{}, err
	}
	cr := CommitRange{
		Target: tc,
		Base:   bc,
	}
	cr.Commits, cr.Ancestor, cr.Linear, err = r.listCommits(target, base)
	if err != nil {
		return CommitRange{}, err
	}
	return cr, nil
}

func (r *GitRepo) listCommits(targetCommit plumbing.Hash, baseCommit plumbing.Hash) ([]*object.Commit, *object.Commit, bool, error) {

	targetCommits, err := r.listCommitHistory(targetCommit, 100, baseCommit)
	if err != nil {
		return nil, nil, false, err
	}

	// if the last element in the targetCommits is the baseCommit, history is linear
	// and we're done
	if targetCommits[len(targetCommits)-1].Hash == baseCommit {
		return targetCommits, targetCommits[len(targetCommits)-1], true, nil
	}

	baseCommits, err := r.listCommitHistory(baseCommit, 100, plumbing.ZeroHash)
	if err != nil {
		return nil, nil, false, err
	}

	var commits []*object.Commit
	var commonParent *object.Commit

	for i := 0; i < len(targetCommits) && commonParent == nil; i++ {
		commits = append(commits, targetCommits[i])
		for j := 0; j < len(baseCommit); j++ {
			if targetCommits[i].Hash == baseCommits[j].Hash {
				commonParent = targetCommits[i]
				break
			}
		}
	}

	return commits, commonParent, false, nil
}

func (r *GitRepo) listCommitHistory(hash plumbing.Hash, n int, untilHash plumbing.Hash) ([]*object.Commit, error) {
	baseCommit, err := r.Repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}

	// get n commits starting with base and going back in time. if base was
	// rebased, then there will be a few diverging commits between target and base
	var baseCommits []*object.Commit
	commit := baseCommit
	for i := 0; i < n; i++ {
		baseCommits = append(baseCommits, commit)
		if untilHash != plumbing.ZeroHash && commit.Hash == untilHash {
			break
		}
		if commit.NumParents() == 0 {
			break
		}
		commit, err = commit.Parent(0)
		if err != nil {
			break
		}
	}
	return baseCommits, nil
}

func (r *GitRepo) HeadBranch() (string, error) {
	ref, err := r.Repo.Head()
	if err != nil {
		return "", err
	}
	return ref.Name().Short(), nil
}

func (r *GitRepo) Fetch() error {
	out, err := r.GitExec("fetch", "origin")
	if err != nil {
		log.Print(out)
		return fmt.Errorf("can't fetch from origin %v", err)
	}
	return nil
}

func (r *GitRepo) Lsremote() ([]*plumbing.Reference, error) {
	out, err := r.GitExec("ls-remote", "-q")
	if err != nil {
		log.Print(out)
		return nil, fmt.Errorf("can't fetch from origin %v", err)
	}
	lines := strings.Split(out, "\n")
	refs := make([]*plumbing.Reference, len(lines))
	for i, l := range lines {
		f := strings.Split(l, "\t")
		if len(f) != 2 {
			return nil, fmt.Errorf("expected two fields for lsremote, got %v", l)
		}
		refs[i] = plumbing.NewHashReference(plumbing.ReferenceName(f[1]),
			plumbing.NewHash(f[0]))
	}
	return refs, nil
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

	client, err := model.NewClient(ctx, workdir)

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

	if name == "" || owner == "" {
		return nil, fmt.Errorf("no remote on github.com found")
	}

	r, _, err := client.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Github repo: %v", err)
	}

	return &GitRepo{
		GitCommand: gitcmd,
		WorkDir:    workdir,
		GitDir:     gitdir,
		Repo:       repo,
		Client:     client,
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

// IsPullRequestRef returns the PR number parsed from the ref and true
// when the ref starts with "refs/pull/"
func IsPullRequestRef(ref *plumbing.Reference) (int, bool) {
	const prefix = "refs/pull/"
	if !strings.HasPrefix(ref.Name().String(), prefix) {
		return 0, false
	}
	prNumStr := ref.Name().String()[len(prefix):]
	prNumStr = prNumStr[0:strings.Index(prNumStr, "/")]
	prNum, err := strconv.Atoi(prNumStr)
	if err != nil {
		return 0, false
	}
	return prNum, true
}
