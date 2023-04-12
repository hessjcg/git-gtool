package gitrepo

import (
	"os/exec"
	"strings"
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
