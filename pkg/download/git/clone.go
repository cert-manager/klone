package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go418/klone/pkg/mod"
)

type cleanup func()

func Get(targetPath string, src mod.KloneSource) (string, error) {
	fmt.Println("Cloning", src.RepoPath, " from ", src.RepoURL, "to", targetPath, "on commit", src.RepoHash)

	if err := sparseCheckout(targetPath, src.RepoURL, src.RepoHash, []string{src.RepoPath}); err != nil {
		return "", err
	}

	return filepath.Join(targetPath, src.RepoPath), nil
}

func runGitCmd(root string, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)

	cmd.Dir = root
	cmd.Env = append(os.Environ(), cmd.Env...)
	// Disable Git terminal prompts in case we're running with a tty
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=false")

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git command failed: %v", err)
	}

	return nil
}

func sparseCheckout(root string, repoURL string, branch string, patterns []string) error {
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("unable to clean repo at %s: %v", root, err)
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	if err := runGitCmd(root, os.Stdout, os.Stderr, "clone", "--depth=1", "--filter=blob:none", "--no-checkout", repoURL, "."); err != nil {
		return err
	}

	if err := runGitCmd(root, os.Stdout, os.Stderr, "config", "advice.detachedHead", "false"); err != nil {
		return err
	}

	if err := runGitCmd(root, os.Stdout, os.Stderr, "sparse-checkout", "init", "--cone", "--sparse-index"); err != nil {
		return err
	}

	args := append([]string{"sparse-checkout", "set"}, patterns...)
	if err := runGitCmd(root, os.Stdout, os.Stderr, args...); err != nil {
		return err
	}

	if err := runGitCmd(root, os.Stdout, os.Stderr, "checkout", branch); err != nil {
		return err
	}

	buffer := bytes.Buffer{}
	if err := runGitCmd(root, &buffer, os.Stderr, "rev-parse", "HEAD"); err != nil {
		return err
	}

	return nil
}
