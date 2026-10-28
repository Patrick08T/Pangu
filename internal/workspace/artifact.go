package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

type Change struct {
	Path, Mode, OldMode, Object, OldObject, Status string
	Size, OldSize                                  int64
}

type Fileset struct {
	Base, Head string
	Changes    []Change
	repository string
}

func Files(ctx context.Context, id string) (*Fileset, error) {
	record, err := Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.ExpiredAt != nil {
		return nil, ErrExpired
	}
	root, err := workspaceRoot()
	if err != nil || !record.Ready || record.WorkerID != WorkerID() || record.Directory != filepath.Join(root, id) {
		return nil, errors.New("产物工作区不可用或属于其他 Worker/磁盘")
	}
	repo := filepath.Join(record.Directory, "base")
	resolved, err := existingPath(repo)
	if err != nil || !samePath(repo, resolved) {
		return nil, errors.New("产物基准目录已移动或被替换")
	}
	data, err := git(ctx, repo, "diff", "--raw", "-z", "--no-abbrev", "--no-renames", "--no-ext-diff", record.BaseCommit, record.HeadCommit, "--")
	if err != nil {
		return nil, err
	}
	out := &Fileset{Base: record.BaseCommit, Head: record.HeadCommit, repository: repo, Changes: []Change{}}
	if data == "" {
		return out, nil
	}
	parts := strings.Split(strings.TrimSuffix(data, "\x00"), "\x00")
	if len(parts)%2 != 0 || len(parts) > 2000 {
		return nil, errors.New("差异格式无效或超过 1000 个文件")
	}
	for i := 0; i < len(parts); i += 2 {
		change, err := out.change(ctx, parts[i], parts[i+1])
		if err != nil {
			return nil, err
		}
		out.Changes = append(out.Changes, change)
	}
	return out, nil
}

func (set *Fileset) change(ctx context.Context, header, path string) (Change, error) {
	meta := strings.Fields(strings.TrimPrefix(header, ":"))
	if len(meta) != 5 {
		return Change{}, errors.New("Git 差异记录无效")
	}
	out := Change{Path: path, OldMode: meta[0], Mode: meta[1], OldObject: meta[2], Object: meta[3], Status: meta[4]}
	var err error
	if out.Mode != "000000" {
		out.Size, err = set.size(ctx, out.Object)
	}
	if err == nil && out.OldMode != "000000" {
		out.OldSize, err = set.size(ctx, out.OldObject)
	}
	return out, err
}

func (set *Fileset) size(ctx context.Context, object string) (int64, error) {
	if !commitID.MatchString(object) {
		return 0, errors.New("Git 对象无效")
	}
	value, err := git(ctx, set.repository, "cat-file", "-s", object)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, 64)
}

func (set *Fileset) Blob(ctx context.Context, object string) ([]byte, error) {
	size, err := set.size(ctx, object)
	if err != nil {
		return nil, err
	}
	if size > 10*1024*1024 {
		return nil, errors.New("文件超过 10 MiB 导出上限")
	}
	return gitBytes(ctx, set.repository, nil, "cat-file", "blob", object)
}

func (set *Fileset) Diff(ctx context.Context, path string) ([]byte, bool, error) {
	return gitOutput(ctx, set.repository, nil, 1024*1024, "diff", "--no-ext-diff", "--no-textconv", "--no-color", set.Base, set.Head, "--", path)
}
