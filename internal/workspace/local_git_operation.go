package workspace

import (
	"context"
	"errors"
	"os"
	"sync"
)

var errLocalGitOperationBlocked = errors.New("本地 Git 操作隔离尚未解除，请确认全部进程停止及仓库完整性后显式恢复")

type localGitOperationKey struct{}

type localGitOperation struct {
	root      *os.Root
	path      string
	name      string
	rootInfo  os.FileInfo
	guardInfo os.FileInfo
	payload   []byte
	mu        sync.Mutex
	unknown   error
	closed    bool
}

func beginLocalGitOperation(ctx context.Context) (context.Context, func(error) error, error) {
	if err := checkLocalGitOperation(ctx); err != nil {
		return ctx, nil, err
	}
	if _, ok := ctx.Value(localGitOperationKey{}).(*localGitOperation); ok {
		return ctx, func(err error) error { return recordLocalGitOperationError(ctx, err) }, nil
	}
	owner, err := createLocalGitOperation()
	if err != nil {
		return ctx, nil, err
	}
	ctx = context.WithValue(ctx, localGitOperationKey{}, owner)
	var once sync.Once
	var result error
	return ctx, func(err error) error {
		once.Do(func() { result = owner.finish(recordLocalGitOperationError(ctx, err)) })
		return errors.Join(err, result)
	}, nil
}

func checkLocalGitOperation(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if owner, ok := ctx.Value(localGitOperationKey{}).(*localGitOperation); ok {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if owner.closed || owner.unknown != nil {
			return errors.Join(errLocalGitOperationBlocked, owner.unknown)
		}
		return owner.checkMarker()
	}
	return checkLocalGitOperationFile()
}

func recordLocalGitOperationError(ctx context.Context, err error) error {
	if owner, ok := ctx.Value(localGitOperationKey{}).(*localGitOperation); ok {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if errors.Is(err, errLocalGitStopUnconfirmed) && owner.unknown == nil {
			owner.unknown = err
		}
		return errors.Join(err, owner.unknown)
	}
	return err
}
