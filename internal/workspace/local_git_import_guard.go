package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

func runLocalImportWithGuard(ctx context.Context, repository, root, parent, ref string, directory *os.Root, m localManifest, probe localCopySpaceProbe, command func(context.Context, string, ...string) *exec.Cmd) (result error) {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return err
	}
	defer func() { result = finish(recordLocalGitOperationError(ctx, result)) }()
	child, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if err := checkLocalGitOperation(child); err != nil {
		return err
	}
	if err := checkLocalCopyReserve(child, repository, probe); err != nil {
		return err
	}
	stop, done := make(chan struct{}), make(chan struct{})
	// Keep monitoring even after cancellation until write/Wait/owned stop all return.
	go watchLocalCopyReserve(context.WithoutCancel(child), repository, probe, cancel, stop, done)
	err = runLocalImportProcess(child, cancel, repository, root, parent, ref, directory, m, command)
	close(stop)
	<-done
	if cause := context.Cause(child); cause != nil {
		return errors.Join(cause, err)
	}
	if err != nil {
		return err
	}
	return checkLocalCopyReserve(child, repository, probe)
}

func runLocalImportProcess(ctx context.Context, cancel context.CancelCauseFunc, repository, root, parent, ref string, directory *os.Root, m localManifest, command func(context.Context, string, ...string) *exec.Cmd) error {
	if err := checkLocalGitOperation(ctx); err != nil {
		return err
	}
	cmd := command(ctx, repository, "fast-import", "--quiet", "--date-format=raw", "--done")
	out, stderr := &diagnostic.Buffer{Limit: 4096}, &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr = out, stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stopLocalImportInputOnCancel(ctx, input)()
	stop, err := startLocalGit(cmd)
	if err != nil {
		input.Close()
		return err
	}
	streamErr := writeLocalImport(ctx, input, repository, root, parent, ref, directory, m)
	if closeErr := input.Close(); streamErr == nil {
		streamErr = closeErr
	}
	if streamErr != nil {
		cancel(streamErr)
	}
	waitErr := errors.Join(cmd.Wait(), stop())
	if cause := context.Cause(ctx); cause != nil {
		return errors.Join(cause, waitErr)
	}
	if waitErr != nil {
		return localGitError(repository, stderr, waitErr)
	}
	return nil
}

func stopLocalImportInputOnCancel(ctx context.Context, input io.Closer) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); _ = input.Close() })
	return func() {
		if !stop() {
			<-done
		}
	}
}
