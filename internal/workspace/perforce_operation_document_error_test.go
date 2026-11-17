package workspace

import (
	"context"
	"errors"
	"testing"
)

func TestLocalGitOperationDocumentSizePreservesTypedLocalCause(t *testing.T) {
	repository, _, object, _ := localOperationOutputFixture(t)
	ctx, finish, err := beginLocalGitOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("first actual operation failure")
	recordLocalGitOperationError(ctx, errors.Join(cause, errLocalGitStopUnconfirmed))
	set := &Fileset{repository: repository, localPerforce: true}
	err = set.documentSize(ctx, object)
	if !errors.Is(err, errLocalGitStopUnconfirmed) || !errors.Is(err, cause) {
		t.Errorf("local Document size lost sticky typed cause: %v", err)
	}
	_ = finish(nil)
	err = set.documentSize(context.Background(), object)
	if !errors.Is(err, errLocalGitOperationBlocked) {
		t.Errorf("fresh Document size lost persistent quarantine cause: %v", err)
	}
}

func TestLocalGitOperationDocumentSizePreservesGenericErrorContract(t *testing.T) {
	repository, _, _, _ := localOperationOutputFixture(t)
	set := &Fileset{repository: repository}
	err := set.documentSize(context.Background(), "invalid-object")
	if err == nil || err.Error() != "资料不可读或超过 64 KiB" {
		t.Fatalf("ordinary Document size changed error text: %v", err)
	}
}
