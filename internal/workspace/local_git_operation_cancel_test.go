package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestLocalGitOperationPreservesCancellationCause(t *testing.T) {
	root := localOperationFixture(t)
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("caller reserve check failed")
	cancel(cause)
	if _, _, err := beginLocalGitOperation(ctx); !errors.Is(err, cause) {
		t.Fatalf("pre-canceled operation lost first caller cause: %v", err)
	}
	if len(localOperationEntries(t, root)) != 0 {
		t.Fatal("pre-canceled operation wrote a guard")
	}
}
