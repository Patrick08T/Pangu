package workspace

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func localObjectDirectory(ctx context.Context, name string) (fs.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeType != os.ModeDir {
		return nil, errors.New("本地对象目录含链接或特殊文件")
	}
	actual, err := existingPath(name)
	if err != nil || !samePath(actual, name) {
		return nil, errors.New("本地对象目录身份或祖先发生跳转")
	}
	return info, ctx.Err()
}

func openLocalObjectStore(ctx context.Context, source string) (*os.Root, fs.FileInfo, error) {
	if !filepath.IsAbs(source) {
		return nil, nil, errors.New("本地对象来源必须为绝对路径")
	}
	for _, name := range []string{source, filepath.Join(source, ".git"), filepath.Join(source, ".git", "objects")} {
		if _, err := localObjectDirectory(ctx, name); err != nil {
			return nil, nil, err
		}
	}
	for _, name := range []string{"commondir", "shallow"} {
		if _, err := os.Lstat(filepath.Join(source, ".git", filepath.FromSlash(name))); !os.IsNotExist(err) {
			return nil, nil, errors.New("本地对象仓库不能借用对象或共享Git目录")
		}
	}
	root := filepath.Join(source, ".git", "objects")
	info, err := localObjectDirectory(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	actual, err := directory.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		directory.Close()
		return nil, nil, errors.New("本地对象根目录已替换")
	}
	return directory, info, nil
}

func inspectLocalObjectStore(ctx context.Context, source string) (localObjectStore, error) {
	return inspectLocalObjectStoreWithLimits(ctx, source, localLimits)
}

func inspectLocalObjectStoreWithLimits(ctx context.Context, source string, limits localGitLimits) (localObjectStore, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	directory, identity, err := openLocalObjectStore(ctx, source)
	if err != nil {
		return localObjectStore{}, err
	}
	defer directory.Close()
	store := localObjectStore{root: filepath.Join(source, ".git", "objects"), identity: identity, files: map[string]fs.FileInfo{}}
	visited := 0
	if err := scanLocalObjectDirectory(ctx, directory, ".", 0, &visited, &store, limits); err != nil {
		return localObjectStore{}, err
	}
	metadata, err := inspectLocalGitMetadata(ctx, source, &visited, store.paths+store.directoryPaths, limits)
	if err != nil {
		return localObjectStore{}, err
	}
	store.metadata = &metadata
	current, err := localObjectDirectory(ctx, store.root)
	if err != nil {
		return localObjectStore{}, err
	}
	if !os.SameFile(identity, current) {
		return localObjectStore{}, errors.New("本地对象根目录在统计期间变化")
	}
	return store, ctx.Err()
}

func scanLocalObjectDirectory(ctx context.Context, root *os.Root, relative string, depth int, visited *int, store *localObjectStore, limits localGitLimits) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 64 {
		return errors.New("本地对象目录超过深度预算")
	}
	info, err := localObjectDirectory(ctx, filepath.Join(store.root, relative))
	if err != nil {
		return err
	}
	directory, err := root.Open(relative)
	if err != nil {
		return err
	}
	defer directory.Close()
	actual, err := directory.Stat()
	if err != nil || !os.SameFile(info, actual) {
		return errors.New("本地对象目录在打开期间变化")
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, readErr := directory.ReadDir(128)
		for _, entry := range entries {
			*visited++
			if *visited > limits.entries {
				return errors.New("本地对象目录超过条数预算")
			}
			name := filepath.Join(relative, entry.Name())
			if len(name) > limits.record {
				return errors.New("本地对象路径超过单条预算")
			}
			if err := inspectLocalObjectEntry(ctx, root, name, depth, visited, store, limits); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func inspectLocalObjectEntry(ctx context.Context, root *os.Root, name string, depth int, visited *int, store *localObjectStore, limits localGitLimits) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if store.metadataScan && (name == "objects" || runtime.GOOS == "windows" && strings.EqualFold(name, "objects")) {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(name), ".promisor") || strings.EqualFold(filepath.Base(name), "alternates") || strings.EqualFold(filepath.Base(name), "http-alternates") {
		return errors.New("本地对象仓库不能借用外部对象")
	}
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if store.metadataScan && (strings.EqualFold(name, "HEAD") || strings.EqualFold(name, "config") || strings.EqualFold(name, "config.worktree") || strings.EqualFold(name, "packed-refs")) && !info.Mode().IsRegular() {
		return errors.New("本地Git元数据必须为普通文件")
	}
	key := filepath.ToSlash(name)
	if uint64(len(key)) > uint64(limits.paths)-store.paths-store.directoryPaths {
		return errors.New("本地对象清单超过路径预算")
	}
	if info.Mode()&os.ModeType == os.ModeDir {
		store.directories++
		store.directoryPaths += uint64(len(key))
		return scanLocalObjectDirectory(ctx, root, name, depth+1, visited, store, limits)
	}
	if !info.Mode().IsRegular() {
		return errors.New("本地对象仓库包含链接或特殊文件")
	}
	actual, err := existingPath(filepath.Join(store.root, name))
	if err != nil || !samePath(actual, filepath.Join(store.root, name)) {
		return errors.New("本地对象文件被替换或跳转")
	}
	if info.Size() < 0 || uint64(info.Size()) > math.MaxUint64-store.bytes {
		return errors.New("本地对象大小溢出")
	}
	if len(store.files) >= limits.entries {
		return errors.New("本地对象清单超过文件或路径预算")
	}
	store.files[key] = info
	store.bytes += uint64(info.Size())
	store.paths += uint64(len(key))
	return nil
}
