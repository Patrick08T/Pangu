package workspace

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func copyTestStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"aa/bb": "raw", "pack/p.pack": "1234567", "pack/p.idx": "index"} {
		localTestWrite(t, root, ".git/objects/"+name, []byte(body))
	}
	return root
}

func TestPerforceCopyCountsWholeObjectStoreNotCurrentTree(t *testing.T) {
	root := copyTestStore(t)
	store, err := inspectLocalObjectStoreWithLimits(context.Background(), root, localLimits)
	if err != nil || store.bytes != 15 || store.paths != 26 || len(store.files) != 3 {
		t.Fatalf("object bytes omitted: bytes=%d paths=%d files=%d err=%v", store.bytes, store.paths, len(store.files), err)
	}
	for name, before := range store.files {
		after, err := os.Lstat(filepath.Join(root, ".git", "objects", name))
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("lost source identity %s: %v", name, err)
		}
	}
}

func TestPerforceCopyCapacityIncludesObjectsNextSnapshotAndOneReserve(t *testing.T) {
	store := localObjectStore{bytes: 15, paths: 26, files: map[string]fs.FileInfo{"a": nil, "b": nil, "c": nil}}
	next := localManifest{Files: []localFileInfo{{}}, TotalBytes: 10, PathBytes: 4}
	need, err := localObjectCopyRequired(store, next, 100<<30)
	if err != nil || need != uint64(10<<30)+16483 {
		t.Fatalf("history/next/reserve not fully admitted: %d %v", need, err)
	}
	store.bytes = math.MaxUint64
	if _, err := localObjectCopyRequired(store, next, 100<<30); err == nil {
		t.Fatal("overflow admitted as small copy")
	}
}

func TestPerforceCopyScanRejectsBorrowedStoresAndBudgets(t *testing.T) {
	for _, marker := range []string{".git/objects/info/alternates", ".git/objects/info/http-alternates", ".git/commondir", ".git/objects/pack/p.promisor"} {
		t.Run(marker, func(t *testing.T) {
			root := copyTestStore(t)
			localTestWrite(t, root, marker, []byte("outside"))
			if _, err := inspectLocalObjectStoreWithLimits(context.Background(), root, localLimits); err == nil {
				t.Fatal("borrowed object store accepted")
			}
		})
	}
	root := copyTestStore(t)
	for _, limits := range []localGitLimits{{1 << 20, 1 << 20, 2, 256 << 20}, {1 << 20, 1 << 20, 20, 5}, {4, 1 << 20, 20, 256 << 20}} {
		if _, err := inspectLocalObjectStoreWithLimits(context.Background(), root, limits); err == nil {
			t.Fatal("partial quota scan accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspectLocalObjectStoreWithLimits(ctx, root, localLimits); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
}
