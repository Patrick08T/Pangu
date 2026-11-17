package perforce

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

type localFileLimits struct{ record, command, files, paths int }

type localFileSet struct {
	snapshot Snapshot
	limits   localFileLimits
	paths    map[string]struct{}
	bytes    int
	excluded int
	ctx      context.Context
}

func (s *localFileSet) add(row Record) error {
	if row["clientFile"] == "" {
		return nil
	}
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	name, excluded, err := s.snapshot.inventoryName(ctx, row["clientFile"])
	if err != nil {
		return err
	}
	if excluded {
		s.excluded++
		return nil
	}
	if _, exists := s.paths[name]; exists {
		return nil
	}
	if len(s.paths) >= s.limits.files {
		return errors.New("Perforce 文件清单唯一文件超过 200 万")
	}
	if len(name) > s.limits.paths-s.bytes {
		return errors.New("Perforce 文件清单规范路径超过 256MiB")
	}
	// localName may return a substring of a large JSON field; retain only the path.
	s.paths[strings.Clone(name)] = struct{}{}
	s.bytes += len(name)
	return nil
}

func (s Snapshot) localFiles(ctx context.Context, limits localFileLimits) ([]string, error) {
	result, err := s.localInventory(ctx, limits, defaultInventoryBudgets())
	return result.Files, err
}

func (s *localCommandSession) stream(ctx context.Context, limits localFileLimits, consume func(Record) error, args ...string) error {
	return runLocalStream(ctx, limits, consume, func(child context.Context) (*exec.Cmd, error) { return s.process(child, args...) })
}

func runLocalStream(ctx context.Context, limits localFileLimits, consume func(Record) error, process func(context.Context) (*exec.Cmd, error)) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd, err := process(child)
	if err != nil {
		return err
	}
	stdoutDiagnostic := &diagnostic.Buffer{Limit: 4096}
	visit := func(row Record) error {
		if row["data"] != "" {
			_, _ = stdoutDiagnostic.Write([]byte(row["data"]))
		}
		return consume(row)
	}
	out := &recordStream{recordLimit: limits.record, totalLimit: limits.command, cancel: cancel, consume: visit}
	stderr := &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr = out, stderr
	runErr := cmd.Run()
	if out.err != nil {
		return out.err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = out.finish(); err != nil {
		return err
	}
	if runErr != nil {
		_, _ = stderr.Write(stdoutDiagnostic.Data.Bytes())
		message, _ := diagnostic.Text(stderr.Data.String(), 4096)
		return fmt.Errorf("Perforce 操作失败: %w；%s", runErr, message)
	}
	return nil
}
