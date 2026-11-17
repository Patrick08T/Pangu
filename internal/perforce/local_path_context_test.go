package perforce

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLocalInventoryPathNormalizationObservesCancellationBeforeTraversal(t *testing.T) {
	s := Snapshot{Config: Config{Scope: "项目"}, Client: "daily", LocalRoot: t.TempDir()}
	ctx := &localCancelContext{Context: t.Context(), remaining: 4}
	path := "//daily/项目/" + strings.Repeat("dir/", 20) + "repo/.git/%zz"
	_, excluded, err := s.inventoryName(ctx, path)
	if excluded || !errors.Is(err, context.Canceled) {
		t.Fatalf("normalization ignored cancellation: excluded=%v err=%v", excluded, err)
	}
}
