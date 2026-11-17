package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/google/uuid"
)

// The caller holds the local worker lock. Only this invocation's unpublished
// directory can be removed; an inherited incomplete directory is not proof of stop.
func publishLocalPreparation(ctx context.Context, s *perforce.Snapshot, directory, expected string, prepare func(string) (string, error)) (commit string, result error) {
	if err := checkLocalGitOperation(ctx); err != nil {
		return "", err
	}
	root, err := openLocalPreparationRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	orphan, err := findLocalPreparation(ctx, root)
	if err != nil {
		return "", err
	}
	if orphan != "" {
		commit, err := readPublishedLocalCapture(ctx, s, filepath.Join(directory, orphan))
		if err != nil {
			return "", fmt.Errorf("先前本地快照准备未确认完成，保留证据并停止新建副本: %w", err)
		}
		if expected != "" && commit != expected {
			return "", errors.New("先前准备目录不属于当前不可变基准")
		}
		if err := publishLocalPreparedPath(root, orphan); err != nil {
			return "", err
		}
		return commit, nil
	}
	name := "preparing-local-" + uuid.NewString()
	defer func() { result = cleanupLocalOperationPreparation(ctx, root, name, result) }()
	commit, err = prepare(filepath.Join(directory, name))
	if err != nil {
		return "", err
	}
	if !commitID.MatchString(commit) || (expected != "" && commit != expected) {
		return "", errors.New("准备结果缺少匹配的有效快照修订")
	}
	if err := checkLocalGitOperation(ctx); err != nil {
		return "", err
	}
	marker, err := json.Marshal(localCaptureMarker{RunID: s.RunID, Commit: commit})
	if err != nil {
		return "", err
	}
	if err := root.WriteFile(filepath.Join(name, ".git", "pangu-local-capture.json"), marker, 0600); err != nil {
		return "", err
	}
	if err := checkLocalGitOperation(ctx); err != nil {
		return "", err
	}
	if err := publishLocalPreparedPath(root, name); err != nil {
		return "", err
	}
	return commit, nil
}

func publishLocalPreparedPath(root *os.Root, name string) error {
	if _, err := root.Lstat("base"); !os.IsNotExist(err) {
		return errors.New("发布目标已存在或状态无法确认")
	}
	return root.Rename(name, "base")
}

func openLocalPreparationRoot(directory string) (*os.Root, error) {
	actual, err := canonicalTarget(directory)
	if err != nil || !filepath.IsAbs(directory) || !samePath(actual, directory) {
		return nil, errors.New("本地快照准备目录发生跳转")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	actual, err = existingPath(directory)
	if err != nil || !samePath(actual, directory) {
		return nil, errors.New("本地快照准备目录不安全")
	}
	return os.OpenRoot(directory)
}

func findLocalPreparation(ctx context.Context, root *os.Root) (string, error) {
	directory, err := root.Open(".")
	if err != nil {
		return "", err
	}
	defer directory.Close()
	var found string
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		entries, readErr := directory.ReadDir(128)
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "preparing-local-") {
				continue
			}
			id, err := uuid.Parse(strings.TrimPrefix(entry.Name(), "preparing-local-"))
			if err != nil || entry.Name() != "preparing-local-"+id.String() || found != "" {
				return "", errors.New("本地快照存在歧义准备目录，停止创建额外副本")
			}
			found = entry.Name()
		}
		if errors.Is(readErr, io.EOF) {
			return found, nil
		}
		if readErr != nil {
			return "", readErr
		}
	}
}

func cleanupLocalPreparation(root *os.Root, name string, cause error) error {
	if cause == nil || errors.Is(cause, errLocalGitStopUnconfirmed) {
		return cause
	}
	id, err := uuid.Parse(strings.TrimPrefix(name, "preparing-local-"))
	if err != nil || name != "preparing-local-"+id.String() {
		return errors.Join(cause, errors.New("拒绝清理非本次准备目录"))
	}
	return errors.Join(cause, root.RemoveAll(name))
}
