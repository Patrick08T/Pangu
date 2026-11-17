package perforce

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
)

type localCancelContext struct {
	context.Context
	remaining int
}

func (c *localCancelContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestLocalInventorySortIsCompleteAndCooperativelyCancelable(t *testing.T) {
	paths := map[string]struct{}{}
	for i := 4095; i >= 0; i-- {
		paths[fmt.Sprintf("file-%04d", i)] = struct{}{}
	}
	got, err := localSortedPaths(t.Context(), paths)
	if err != nil || len(got) != 4096 || got[0] != "file-0000" || got[4095] != "file-4095" || !slices.IsSorted(got) {
		t.Fatalf("incomplete sort: n=%d err=%v", len(got), err)
	}
	ctx := &localCancelContext{Context: t.Context(), remaining: 4104}
	got, err = localSortedPaths(ctx, paths)
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("sort ignored cancellation: n=%d err=%v", len(got), err)
	}
}
