package workspace

import (
	"context"
	"errors"
	"os"
)

func sameLocalObjectVersion(ctx context.Context, before, after localObjectStore) error {
	if !os.SameFile(before.identity, after.identity) || before.bytes != after.bytes || before.paths != after.paths || len(before.files) != len(after.files) || before.directories != after.directories || before.directoryPaths != after.directoryPaths {
		return errors.New("本地对象或Git元数据来源在复制期间变化")
	}
	for name, original := range before.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := after.files[name]
		if current == nil || !os.SameFile(original, current) || original.Size() != current.Size() || original.Mode() != current.Mode() || !original.ModTime().Equal(current.ModTime()) {
			return errors.New("本地对象或Git元数据文件在复制期间变化")
		}
	}
	return ctx.Err()
}

func verifyLocalObjectCopy(ctx context.Context, source, target string, before localObjectStore) error {
	after, err := inspectLocalObjectStore(ctx, source)
	if err != nil {
		return err
	}
	if err := sameLocalObjectVersion(ctx, before, after); err != nil {
		return err
	}
	if before.metadata == nil || after.metadata == nil {
		return errors.New("本地Git元数据来源缺少完整证明")
	}
	if err := sameLocalObjectVersion(ctx, *before.metadata, *after.metadata); err != nil {
		return err
	}
	copied, err := inspectLocalObjectStore(ctx, target)
	if err != nil {
		return err
	}
	if os.SameFile(after.identity, copied.identity) || after.bytes != copied.bytes || after.paths != copied.paths || len(after.files) != len(copied.files) {
		return errors.New("本地对象复制不完整或共用目录")
	}
	for name, original := range after.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := copied.files[name]
		if current == nil || os.SameFile(original, current) || original.Size() != current.Size() {
			return errors.New("本地对象文件未独立完整复制")
		}
	}
	return ctx.Err()
}
