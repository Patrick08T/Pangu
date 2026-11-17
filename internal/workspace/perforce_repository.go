package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/perforce"
)

type localCaptureMarker struct {
	RunID  string `json:"run_id"`
	Commit string `json:"commit"`
}

func initializeLocalRepository(ctx context.Context, repository string) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	if err := os.MkdirAll(repository, 0700); err != nil {
		return "", err
	}
	if _, err := runLocalGit(ctx, repository, "init", "--initial-branch=main"); err != nil {
		return "", err
	}
	if _, err := runLocalGit(ctx, repository, "commit", "--allow-empty", "-m", "Pangu 本地 P4 快照根"); err != nil {
		return "", err
	}
	return runLocalGit(ctx, repository, "rev-parse", "HEAD")
}

func cloneLocalRepository(ctx context.Context, source, commit, target string, next localManifest) (result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	if !filepath.IsAbs(source) || !filepath.IsAbs(target) || !commitID.MatchString(commit) {
		return errors.New("本地快照复制参数无效")
	}
	actual, err := existingPath(source)
	if err != nil || !samePath(actual, source) {
		return errors.New("本地快照来源被替换或跳转")
	}
	actual, err = canonicalTarget(target)
	if err != nil || !samePath(actual, target) {
		return errors.New("本地快照复制目标发生跳转")
	}
	if relative, err := filepath.Rel(source, target); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("运行目录不得位于源项目内")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("本地快照复制目标必须尚不存在")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// The copy validates source layout/config before Git reads it, budgets all
	// historical objects, and confirms descendants have stopped on cancellation.
	if err := cloneLocalObjectStore(ctx, source, target, next); err != nil {
		return err
	}
	if err := inspectCompleteLocalRepository(ctx, target, commit); err != nil {
		return err
	}
	if _, err := runLocalGit(ctx, target, "remote", "remove", "origin"); err != nil {
		return err
	}
	if _, err := runLocalGit(ctx, target, "update-ref", "HEAD", commit); err != nil {
		return err
	}
	return checkLocalCaptureCapacity(ctx, target, next)
}

func localObjectSource(directory string) string {
	base := filepath.Join(directory, "base")
	if _, err := os.Lstat(base); !os.IsNotExist(err) {
		return base
	}
	return filepath.Join(directory, "source")
}

func prepareLocalObjectRepository(ctx context.Context, s *perforce.Snapshot, directory, previous, project string) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	if s.Commit != "" {
		if err := inspectCompleteLocalRepository(ctx, localObjectSource(directory), s.Commit); err != nil {
			return "", err
		}
		return s.Commit, nil
	}
	base := filepath.Join(directory, "base")
	if _, err := os.Lstat(base); err == nil {
		return readPublishedLocalCapture(ctx, s, base)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return publishLocalPreparation(ctx, s, directory, "", func(staging string) (string, error) {
		return importLocalSource(ctx, s, staging, previous, project)
	})
}

func readPublishedLocalCapture(ctx context.Context, s *perforce.Snapshot, base string) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	if _, err := inspectLocalObjectStore(ctx, base); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return "", err
	}
	defer root.Close()
	file, err := root.Open(filepath.Join(".git", "pangu-local-capture.json"))
	if err != nil {
		return "", errors.New("未登记的本地快照缺少完整发布标记")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	var marker localCaptureMarker
	if len(data) > 4096 || json.Unmarshal(data, &marker) != nil || marker.RunID != s.RunID || !commitID.MatchString(marker.Commit) {
		return "", errors.New("本地快照发布标记不匹配")
	}
	if _, err := verifyLocalBinding(ctx, s); err != nil {
		return "", err
	}
	if err := inspectLocalRepository(ctx, base, marker.Commit); err != nil {
		return "", err
	}
	if _, err := runLocalGit(ctx, base, localObjectCheckArgs(marker.Commit)...); err != nil {
		return "", err
	}
	head, err := runLocalGit(ctx, base, "rev-parse", "HEAD")
	if err != nil || head != marker.Commit {
		return "", errors.New("本地快照发布标记与 HEAD 不一致")
	}
	return marker.Commit, nil
}

func inspectCompleteLocalRepository(ctx context.Context, repository, commit string) (result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return err
	}
	defer func() { result = finish(result) }()
	if _, err := inspectLocalObjectStore(ctx, repository); err != nil {
		return err
	}
	if err := inspectLocalRepository(ctx, repository, commit); err != nil {
		return err
	}
	_, err = runLocalGit(ctx, repository, localObjectCheckArgs(commit)...)
	return err
}

func sameLocalTrees(ctx context.Context, repository, before, after string) (_ bool, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return false, err
	}
	defer func() { result = finish(result) }()
	if !commitID.MatchString(before) || !commitID.MatchString(after) {
		return false, errors.New("快照修订标识无效")
	}
	left, err := runLocalGit(ctx, repository, "rev-parse", before+"^{tree}")
	if err != nil {
		return false, err
	}
	right, err := runLocalGit(ctx, repository, "rev-parse", after+"^{tree}")
	return left == right, err
}
