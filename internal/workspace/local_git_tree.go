package workspace

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type localTreeCheck struct {
	paths  map[string]bool
	bytes  int
	limits localGitLimits
}

func (s *localTreeCheck) visit(record []byte) error {
	meta, name, ok := strings.Cut(string(record), "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 4 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !commitID.MatchString(fields[2]) {
		return errors.New("本地快照树包含非普通文件或无效对象")
	}
	size, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || size < 0 {
		return errors.New("本地快照树对象不可用")
	}
	if err := validateLocalName(name); err != nil {
		return err
	}
	key := localNameKey(name)
	if s.paths[key] || len(s.paths) >= s.limits.entries || len(name) > s.limits.paths-s.bytes {
		return errors.New("本地快照树路径重复或超过预算")
	}
	s.paths[key], s.bytes = true, s.bytes+len(name)
	return nil
}

func localCommitExists(ctx context.Context, repository, commit string) error {
	if !commitID.MatchString(commit) {
		return errors.New("本地快照提交ID无效")
	}
	kind, err := runLocalGit(ctx, repository, "cat-file", "-t", commit)
	if err != nil {
		return err
	}
	if kind != "commit" {
		return errors.New("本地快照父对象不是提交")
	}
	return nil
}

func inspectLocalRepository(ctx context.Context, repository, commit string) error {
	if err := localCommitExists(ctx, repository, commit); err != nil {
		return err
	}
	check := localTreeCheck{paths: make(map[string]bool), limits: localLimits}
	return streamLocalGit(ctx, repository, []string{"ls-tree", "-r", "-l", "-z", "--full-tree", commit}, check.visit)
}
