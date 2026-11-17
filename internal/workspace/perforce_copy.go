package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type localObjectStore struct {
	root                        string
	identity                    fs.FileInfo
	files                       map[string]fs.FileInfo
	bytes, paths                uint64
	directories, directoryPaths uint64
	metadata                    *localObjectStore
	metadataScan                bool
}

// cloneLocalObjectStore requires the caller to hold the source run lock and lease.
func cloneLocalObjectStore(ctx context.Context, source, target string, next localManifest) error {
	return cloneLocalObjectStoreWithSpace(ctx, source, target, next, localVolumeSpace)
}

func cloneLocalObjectStoreWithSpace(ctx context.Context, source, target string, next localManifest, probe localCopySpaceProbe) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if err := validateLocalObjectTarget(ctx, source, target); err != nil {
		return err
	}
	before, err := inspectLocalObjectStore(ctx, source)
	if err != nil {
		return err
	}
	parent := filepath.Dir(target)
	if err := checkLocalObjectCapacity(ctx, parent, before, next, probe); err != nil {
		return err
	}
	if err := runLocalCopyCommand(ctx, parent, probe, "clone", "--local", "--no-hardlinks", "--no-checkout", "--", source, target); err != nil {
		return err
	}
	return verifyLocalObjectCopy(ctx, source, target, before)
}

func validateLocalObjectTarget(ctx context.Context, source, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) {
		return errors.New("本地对象复制路径必须为绝对路径")
	}
	actual, err := canonicalTarget(target)
	if err != nil || !samePath(actual, target) {
		return errors.New("本地对象复制目标祖先发生跳转")
	}
	if rel, err := filepath.Rel(source, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("本地对象复制目标不能位于来源内部")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("本地对象复制目标必须不存在")
	}
	_, err = localObjectDirectory(ctx, filepath.Dir(target))
	return err
}
