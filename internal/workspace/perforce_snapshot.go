package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Patrick08T/Pangu/internal/perforce"
)

const localSnapshotTimeout = 2 * time.Hour

func importLocalPerforce(ctx context.Context, s *perforce.Snapshot, source string) (string, error) {
	return importInitialLocalSnapshot(ctx, s, source, nil)
}

func localSnapshot(ctx context.Context, s *perforce.Snapshot, repository, parent string) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	root, manifest, err := captureLocalState(ctx, s)
	if err != nil {
		return "", err
	}
	return commitLocalState(ctx, s, repository, parent, root, manifest)
}

func captureLocalState(ctx context.Context, s *perforce.Snapshot) (string, localManifest, error) {
	root, err := verifyLocalBinding(ctx, s)
	if err != nil {
		return "", localManifest{}, err
	}
	inventory, err := s.LocalInventory(ctx)
	if err != nil {
		return "", localManifest{}, err
	}
	manifest, err := inspectLocalFiles(ctx, root, inventory.Files)
	return root, manifest, err
}

func verifyLocalBinding(ctx context.Context, s *perforce.Snapshot) (string, error) {
	path, err := s.LocalDirectory(ctx)
	if err != nil {
		return "", err
	}
	resolved, err := existingPath(path)
	if err != nil || !samePath(resolved, path) {
		return "", errors.New("绑定目录被替换或跳转")
	}
	return path, nil
}

func commitLocalState(ctx context.Context, s *perforce.Snapshot, repository, parent, root string, manifest localManifest) (string, error) {
	commit, err := importLocalFiles(ctx, repository, root, parent, manifest)
	if err != nil {
		return "", err
	}
	current, err := verifyLocalBinding(ctx, s)
	if err != nil {
		return "", err
	}
	if !samePath(current, root) {
		return "", errors.New("捕获期间本地 P4 绑定发生变化")
	}
	return commit, nil
}

func importInitialLocalSnapshot(ctx context.Context, s *perforce.Snapshot, repository string, prior *Run) (_ string, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() { result = finish(result) }()
	ctx, cancel := context.WithTimeout(ctx, localSnapshotTimeout)
	defer cancel()
	if err := os.MkdirAll(filepath.Dir(repository), 0700); err != nil {
		return "", err
	}
	root, manifest, err := captureLocalState(ctx, s)
	if err != nil {
		return "", err
	}
	if err := checkLocalCaptureCapacity(ctx, filepath.Dir(repository), manifest); err != nil {
		return "", err
	}
	var parent string
	if prior == nil {
		parent, err = initializeLocalRepository(ctx, repository)
	} else {
		err = cloneLocalRepository(ctx, filepath.Join(prior.Directory, "base"), prior.HeadCommit, repository, manifest)
		parent = prior.HeadCommit
	}
	if err != nil {
		return "", err
	}
	commit, err := commitLocalState(ctx, s, repository, parent, root, manifest)
	if err == nil {
		_, err = runLocalGit(ctx, repository, "update-ref", "HEAD", commit)
	}
	return commit, err
}

// Kept for existing callers/tests; filesystem validation now honors cancellation.
func localRegularFiles(root string, names []string) ([]string, error) {
	manifest, err := inspectLocalFiles(context.Background(), root, names)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		out = append(out, file.Name)
	}
	return out, nil
}
