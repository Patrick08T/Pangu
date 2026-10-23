package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func sourceRepository(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	if _, err := git(ctx, dir, "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.txt"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, dir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, dir, "commit", "-m", "基准"); err != nil {
		t.Fatal(err)
	}
	commit, err := Resolve(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	return dir, commit
}

func TestCloneIgnoresDirtySourceAndIsIndependent(t *testing.T) {
	source, commit := sourceRepository(t)
	ctx := context.Background()
	_ = os.WriteFile(filepath.Join(source, "source.txt"), []byte("dirty"), 0600)
	_ = os.WriteFile(filepath.Join(source, "untracked.txt"), []byte("private"), 0600)
	target := filepath.Join(t.TempDir(), "clone")
	if err := Clone(ctx, source, commit, target); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(target, "source.txt"))
	if string(data) != "base" {
		t.Fatalf("复制了源工作区修改: %q", data)
	}
	if _, err := os.Stat(filepath.Join(target, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatal("复制了源未跟踪文件")
	}
	if _, err := os.Stat(filepath.Join(target, ".git/objects/info/alternates")); !os.IsNotExist(err) {
		t.Fatal("clone 不应借用源仓库对象")
	}
	data, _ = os.ReadFile(filepath.Join(source, "source.txt"))
	if string(data) != "dirty" {
		t.Fatal("修改了源工作区")
	}
	current, _ := Resolve(ctx, source, "main")
	if current != commit {
		t.Fatal("移动了源分支")
	}
}

func TestCloneRejectsDirectoryAliasIntoSource(t *testing.T) {
	source, commit := sourceRepository(t)
	alias := filepath.Join(t.TempDir(), "alias")
	var err error
	if runtime.GOOS == "windows" {
		err = exec.Command("cmd", "/c", "mklink", "/J", alias, source).Run()
	} else {
		err = os.Symlink(source, alias)
	}
	if err != nil {
		t.Fatalf("无法建立目录别名测试夹具: %v", err)
	}
	if err := Clone(context.Background(), source, commit, filepath.Join(alias, "unexpected")); err == nil {
		t.Fatal("目录别名绕过源项目隔离")
	}
	if _, err := os.Stat(filepath.Join(source, "unexpected")); !os.IsNotExist(err) {
		t.Fatal("源项目被写入")
	}
}

func TestFailedAttemptDoesNotPolluteNextAttempt(t *testing.T) {
	source, commit := sourceRepository(t)
	ctx := context.Background()
	base := filepath.Join(t.TempDir(), "base")
	if err := Clone(ctx, source, commit, base); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "success")
	if err := Clone(ctx, base, commit, first); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(first, "binary.bin"), []byte{0, 255, 3}, 0600)
	_ = os.Remove(filepath.Join(first, "source.txt"))
	next, err := Snapshot(ctx, base, first, commit)
	if err != nil {
		t.Fatal(err)
	}
	failed := filepath.Join(t.TempDir(), "failed")
	if err := Clone(ctx, base, next, failed); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(failed, "leak.txt"), []byte("failed"), 0600)
	retry := filepath.Join(t.TempDir(), "retry")
	if err := Clone(ctx, base, next, retry); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(retry, "leak.txt")); !os.IsNotExist(err) {
		t.Fatal("失败修改污染重试")
	}
	data, _ := os.ReadFile(filepath.Join(retry, "binary.bin"))
	if len(data) != 3 || data[1] != 255 {
		t.Fatal("丢失成功上游二进制文件")
	}
	if _, err := os.Stat(filepath.Join(retry, "source.txt")); !os.IsNotExist(err) {
		t.Fatal("丢失成功上游删除操作")
	}
}

func TestSnapshotRejectsJunctionAndSpecialFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction 专项")
	}
	source, commit := sourceRepository(t)
	base, attempt := filepath.Join(t.TempDir(), "base"), filepath.Join(t.TempDir(), "attempt")
	ctx := context.Background()
	for _, path := range []string{base, attempt} {
		if err := Clone(ctx, source, commit, path); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(attempt, "outside")
	if err := exec.Command("cmd", "/c", "mklink", "/J", alias, source).Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := Snapshot(ctx, base, attempt, commit); err == nil {
		t.Fatal("检查点不应遍历指向外部的 junction")
	}
}
