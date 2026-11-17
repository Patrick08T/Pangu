package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func localTestLargeTree(t *testing.T, repository, parent string) string {
	t.Helper()
	var input strings.Builder
	fmt.Fprintf(&input, "blob\nmark :1\ndata 0\n\ncommit refs/heads/large\ncommitter Test <test@localhost> 0 +0000\ndata 1\nx\nfrom %s\n", parent)
	prefix := strings.Repeat(strings.Repeat("a", 200)+"/", 4)
	for i := 0; i < 18000; i++ {
		fmt.Fprintf(&input, "M 100644 :1 %s%05d%s\n", prefix, i, strings.Repeat("b", 180))
	}
	input.WriteString("\ndone\n")
	cmd := localGitCommand(context.Background(), repository, "fast-import", "--quiet", "--done")
	cmd.Stdin = strings.NewReader(input.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("large tree fixture: %v %s", err, stderr.String())
	}
	commit, err := git(context.Background(), repository, "rev-parse", "large")
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func TestLocalGitInspectsBeyond16MiBAndRejectsLateSymlink(t *testing.T) {
	repository, parent := localTestRepository(t)
	commit := localTestLargeTree(t, repository, parent)
	count, total := 0, 0
	err := streamLocalGit(context.Background(), repository, []string{"ls-tree", "-r", "-l", "-z", commit}, func(p []byte) error { count++; total += len(p) + 1; return nil })
	if err != nil || count != 18000 || total <= 16<<20 {
		t.Fatalf("large output incomplete: %d/%d %v", count, total, err)
	}
	if err := inspectLocalRepository(context.Background(), repository, commit); err != nil {
		t.Fatal(err)
	}
	input := fmt.Sprintf("commit refs/heads/late\ncommitter Test <test@localhost> 0 +0000\ndata 1\nx\nfrom %s\nM 120000 inline zz-link\ndata 6\ntarget\n\ndone\n", commit)
	cmd := localGitCommand(context.Background(), repository, "fast-import", "--quiet", "--done")
	cmd.Stdin = strings.NewReader(input)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	late, err := git(context.Background(), repository, "rev-parse", "late")
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectLocalRepository(context.Background(), repository, late); err == nil {
		t.Fatal("late symlink after 16MiB was ignored")
	}
}

func TestLocalGitStreamPreservesLateVisitorErrorAndCommandFailures(t *testing.T) {
	repository, parent := localTestRepository(t)
	commit := localTestLargeTree(t, repository, parent)
	cause, count := errors.New("trusted late rejection"), 0
	err := streamLocalGit(context.Background(), repository, []string{"ls-tree", "-rz", commit}, func([]byte) error {
		count++
		if count == 17999 {
			return cause
		}
		return nil
	})
	if !errors.Is(err, cause) || count != 17999 {
		t.Fatalf("visitor root cause lost: %d %v", count, err)
	}
	if err := streamLocalGit(context.Background(), repository, []string{"rev-parse", "HEAD"}, func([]byte) error { return nil }); err == nil {
		t.Fatal("non-NUL tail silently accepted")
	}
	if err := streamLocalGit(context.Background(), repository, []string{"not-a-git-command"}, func([]byte) error { return nil }); err == nil {
		t.Fatal("command exit failure ignored")
	}
}

func TestLocalGitCommandBoundsOutputAndIgnoresInheritedGitEnvironment(t *testing.T) {
	repository, parent := localTestRepository(t)
	t.Setenv("GIT_DIR", t.TempDir())
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", t.TempDir())
	got, err := runLocalGit(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil || got != parent {
		t.Fatalf("inherited Git config redirected command: %q %v", got, err)
	}
	if _, err := runLocalGit(context.Background(), repository, "log", "-1", "--format="+strings.Repeat("x", 4097)); err == nil {
		t.Fatal("oversized successful output was silently truncated")
	}
}
