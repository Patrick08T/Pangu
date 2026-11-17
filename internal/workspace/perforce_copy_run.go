package workspace

import (
	"context"
	"errors"
	"time"
)

func checkLocalCopyReserve(ctx context.Context, directory string, probe localCopySpaceProbe) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	free, total, err := probe(directory)
	if err != nil {
		return err
	}
	if free < localReserveSpace(total) {
		return errors.New("本地对象复制剩余空间低于保留余量")
	}
	return context.Cause(ctx)
}

func watchLocalCopyReserve(ctx context.Context, directory string, probe localCopySpaceProbe, cancel context.CancelCauseFunc, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := checkLocalCopyReserve(ctx, directory, probe); err != nil {
				cancel(err)
				return
			}
		}
	}
}

func runLocalCopyCommand(ctx context.Context, directory string, probe localCopySpaceProbe, args ...string) error {
	if err := checkLocalCopyReserve(ctx, directory, probe); err != nil {
		return err
	}
	child, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stop, done := make(chan struct{}), make(chan struct{})
	go watchLocalCopyReserve(child, directory, probe, cancel, stop, done)
	// runLocalGit returns only after cmd.Wait and the owned Job stop confirmation.
	_, err := runLocalGit(child, directory, args...)
	close(stop)
	<-done
	if cause := context.Cause(child); cause != nil {
		return errors.Join(cause, err)
	}
	if err != nil {
		return err
	}
	return checkLocalCopyReserve(ctx, directory, probe)
}
