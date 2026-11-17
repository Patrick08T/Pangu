package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

type localGitLimits struct{ record, total, entries, paths int }

var localLimits = localGitLimits{1 << 20, 512 << 20, 2_000_000, 256 << 20}

func localGitCommand(ctx context.Context, directory string, args ...string) *exec.Cmd {
	options := []string{"--literal-pathspecs", "-c", "core.longpaths=true", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "core.fsmonitor=false", "-c", "core.autocrlf=false", "-c", "core.attributesFile=" + os.DevNull,
		"-c", "user.name=Pangu", "-c", "user.email=pangu@localhost", "-c", "commit.gpgSign=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	cmd := exec.CommandContext(ctx, "git", append(options, args...)...)
	cmd.Dir, cmd.WaitDelay = directory, time.Second
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func runLocalGit(ctx context.Context, directory string, args ...string) (output string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		result = finish(result)
		if result != nil {
			output = ""
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	cmd := localGitCommand(ctx, directory, args...)
	out, stderr := &diagnostic.Buffer{Limit: 4096}, &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr = out, stderr
	err = recordLocalGitOperationError(ctx, runManagedLocalGit(cmd))
	if ctx.Err() != nil {
		return "", errors.Join(ctx.Err(), err)
	}
	if err != nil {
		return "", localGitError(directory, stderr, err)
	}
	if out.Truncated {
		return "", errors.New("本地 Git 诊断输出超过4096字节")
	}
	return strings.TrimSpace(out.Data.String()), nil
}

func localGitError(directory string, stderr *diagnostic.Buffer, err error) error {
	message, _ := diagnostic.Text(strings.ReplaceAll(stderr.Data.String(), directory, "[工作目录]"), 4096)
	return fmt.Errorf("本地 Git 操作失败: %w；%s", err, message)
}

type localNULStream struct {
	ctx          context.Context
	limits       localGitLimits
	visit        func([]byte) error
	cancel       context.CancelFunc
	pending      []byte
	total, count int
	paths        int
	treeRecords  bool
	err          error
}

func (s *localNULStream) Write(data []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if err := s.ctx.Err(); err != nil {
		return 0, s.fail(err)
	}
	if len(data) > s.limits.total-s.total {
		return 0, s.fail(errors.New("本地 Git 输出超过总字节预算"))
	}
	s.total += len(data)
	for rest := data; len(rest) > 0; {
		n := bytes.IndexByte(rest, 0)
		end := n
		if n < 0 {
			end = len(rest)
		}
		if end > s.limits.record-len(s.pending) {
			return 0, s.fail(errors.New("本地 Git 记录超过单条预算"))
		}
		s.pending = append(s.pending, rest[:end]...)
		if n < 0 {
			break
		}
		if s.count >= s.limits.entries {
			return 0, s.fail(errors.New("本地 Git 记录超过数量预算"))
		}
		if err := s.ctx.Err(); err != nil {
			return 0, s.fail(err)
		}
		pathBytes := len(s.pending)
		if s.treeRecords {
			if tab := bytes.IndexByte(s.pending, '\t'); tab >= 0 {
				pathBytes -= tab + 1
			}
		}
		if pathBytes > s.limits.paths-s.paths {
			return 0, s.fail(errors.New("本地 Git 路径超过字节预算"))
		}
		s.paths += pathBytes
		s.count++
		if err := s.visit(s.pending); err != nil {
			return 0, s.fail(err)
		}
		s.pending, rest = s.pending[:0], rest[n+1:]
	}
	return len(data), nil
}

func (s *localNULStream) fail(err error) error {
	if s.err == nil {
		s.err = err
		s.cancel()
	}
	return s.err
}

func streamLocalGit(ctx context.Context, repository string, args []string, visit func([]byte) error) (result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	ctx, timeout := context.WithTimeout(ctx, time.Hour)
	defer timeout()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &localNULStream{ctx: ctx, limits: localLimits, visit: visit, cancel: cancel, treeRecords: len(args) > 0 && args[0] == "ls-tree"}
	stderr := &diagnostic.Buffer{Limit: 4096}
	cmd := localGitCommand(child, repository, args...)
	cmd.Stdout, cmd.Stderr = out, stderr
	err = recordLocalGitOperationError(ctx, runManagedLocalGit(cmd))
	if out.err != nil {
		return errors.Join(out.err, err)
	}
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	if err != nil {
		return localGitError(repository, stderr, err)
	}
	if len(out.pending) != 0 {
		return errors.New("本地 Git 输出包含未结束的NUL记录")
	}
	return nil
}
