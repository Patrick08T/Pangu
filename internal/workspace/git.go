package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/google/uuid"
)

var commitID = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func git(ctx context.Context, directory string, args ...string) (string, error) {
	return gitEnvironment(ctx, directory, nil, args...)
}

func gitEnvironment(ctx context.Context, directory string, extra []string, args ...string) (string, error) {
	data, err := gitBytes(ctx, directory, extra, args...)
	return strings.TrimSpace(string(data)), err
}

func gitBytes(ctx context.Context, directory string, extra []string, args ...string) ([]byte, error) {
	data, truncated, err := gitOutput(ctx, directory, extra, 16*1024*1024, args...)
	if err == nil && truncated {
		err = errors.New("Git 查询结果超过16MiB")
	}
	return data, err
}

func gitOutput(ctx context.Context, directory string, extra []string, limit int, args ...string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	options := []string{"--literal-pathspecs", "-c", "core.longpaths=true", "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false",
		"-c", "user.name=Pangu", "-c", "user.email=pangu@localhost", "-c", "commit.gpgSign=false"}
	cmd := exec.CommandContext(ctx, "git", append(options, args...)...)
	cmd.Dir = directory
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, extra...)
	out, stderr := &diagnostic.Buffer{Limit: limit}, &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = out, stderr, time.Second
	if err := cmd.Run(); err != nil {
		message, _ := diagnostic.Text(strings.ReplaceAll(stderr.Data.String(), directory, "[工作目录]"), 4096)
		return nil, false, fmt.Errorf("Git 操作失败: %w；%s", err, message)
	}
	return out.Data.Bytes(), out.Truncated, nil
}

func Resolve(ctx context.Context, source, branch string) (string, error) {
	if !filepath.IsAbs(source) || branch == "" {
		return "", errors.New("项目需要绝对路径和本地分支")
	}
	if _, err := git(ctx, source, "check-ref-format", "--branch", branch); err != nil {
		return "", errors.New("项目分支名无效")
	}
	value, err := git(ctx, source, "rev-parse", "--verify", "--end-of-options", "refs/heads/"+branch+"^{commit}")
	if err != nil || !commitID.MatchString(value) {
		return "", errors.New("无法解析已登记的本地分支，不自动 fetch")
	}
	return value, nil
}

func Clone(ctx context.Context, source, commit, target string) error {
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || !commitID.MatchString(commit) {
		return errors.New("clone 参数无效")
	}
	var err error
	source, err = existingPath(source)
	if err != nil {
		return err
	}
	target, err = canonicalTarget(target)
	if err != nil {
		return err
	}
	if relative, err := filepath.Rel(source, target); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("运行目录不得位于源项目内")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("clone 目标必须尚不存在")
	}
	if err := Inspect(ctx, source, commit); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	if _, err := git(ctx, filepath.Dir(target), "clone", "--no-local", "--no-hardlinks", "--no-checkout", "--", source, target); err != nil {
		return err
	}
	if _, err := git(ctx, target, "remote", "remove", "origin"); err != nil {
		return err
	}
	_, err = git(ctx, target, "checkout", "--detach", commit, "--")
	return err
}

func canonicalTarget(path string) (string, error) {
	if _, err := os.Lstat(path); err == nil {
		return existingPath(path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", errors.New("目录根不存在")
	}
	resolved, err := canonicalTarget(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func Inspect(ctx context.Context, source, commit string) error {
	data, err := git(ctx, source, "ls-tree", "-rz", commit)
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(data, "\x00") {
		meta, path, _ := strings.Cut(entry, "\t")
		if strings.HasPrefix(meta, "160000 ") {
			return errors.New("首版不支持 Git submodule")
		}
		if filepath.Base(path) == ".gitattributes" {
			attributes, err := git(ctx, source, "show", commit+":"+path)
			if err != nil {
				return err
			}
			if strings.Contains(attributes, "filter=lfs") {
				return errors.New("首版不支持 Git LFS")
			}
		}
	}
	return nil
}

func Snapshot(ctx context.Context, repository, worktree, parent string) (string, error) {
	if !commitID.MatchString(parent) {
		return "", errors.New("检查点父提交无效")
	}
	if err := inspectWorktree(worktree); err != nil {
		return "", err
	}
	index := filepath.Join(repository, ".git", "pangu-index-"+uuid.NewString())
	defer os.Remove(index)
	env := []string{"GIT_INDEX_FILE=" + index}
	options := []string{"--git-dir=" + filepath.Join(repository, ".git"), "--work-tree=" + worktree}
	// 使用 Worker 持有的仓库配置，不能执行 Agent 工作副本中的 hooks 或 filters。
	for _, args := range [][]string{{"read-tree", parent}, {"add", "-A", "--", "."}} {
		if _, err := gitEnvironment(ctx, worktree, env, append(options, args...)...); err != nil {
			return "", err
		}
	}
	tree, err := gitEnvironment(ctx, worktree, env, append(options, "write-tree")...)
	if err != nil {
		return "", err
	}
	commit, err := git(ctx, repository, "commit-tree", tree, "-p", parent, "-m", "Pangu 成功节点检查点")
	if err != nil {
		return "", err
	}
	_, err = git(ctx, repository, "update-ref", "refs/heads/pangu-"+uuid.NewString(), commit)
	return commit, err
}

func inspectWorktree(root string) error {
	resolved, err := existingPath(root)
	if err != nil || !samePath(resolved, filepath.Clean(root)) {
		return errors.New("执行目录被替换为链接，拒绝发布检查点")
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeIrregular != 0 || entry.Type()&os.ModeType != 0 && !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			return errors.New("工作区含目录别名或特殊文件，拒绝发布检查点")
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		return nil
	})
}

func samePath(first, second string) bool {
	relative, err := filepath.Rel(first, second)
	return err == nil && relative == "."
}
