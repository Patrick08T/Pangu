package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPerforceCopyMetadataRejectsExternalConfigBeforeGit(t *testing.T) {
	for _, config := range []string{"[include]\n path = ../outside\n", "[includeIf \"gitdir:*\"]\n path = ../outside\n", "[extensions]\n partialClone = origin\n", "[remote \"origin\"]\n promisor = true\n", "[unknown]\n borrow = outside\n"} {
		t.Run(strings.ReplaceAll(config, "\n", " "), func(t *testing.T) {
			root := copyTestStore(t)
			localTestWrite(t, root, ".git/config", []byte(config))
			if _, err := inspectLocalObjectStore(context.Background(), root); err == nil {
				t.Fatal("external or ambiguous Git config accepted before source Git read")
			}
			target := filepath.Join(t.TempDir(), "copy")
			queried := false
			probe := func(string) (uint64, uint64, error) { queried = true; return 100 << 30, 100 << 30, nil }
			if err := cloneLocalObjectStoreWithSpace(context.Background(), root, target, localManifest{}, probe); err == nil || queried {
				t.Fatalf("unsafe source reached copy admission/process: err=%v queried=%v", err, queried)
			}
		})
	}
}

func TestPerforceCopyMetadataRejectsCaseVariantExternalConfig(t *testing.T) {
	for _, name := range []string{"ConFiG", "CONFIG.worktree"} {
		root := copyTestStore(t)
		localTestWrite(t, root, ".git/"+name, []byte("[include]\n path = ../outside\n"))
		if _, err := inspectLocalObjectStore(context.Background(), root); err == nil {
			t.Fatal("case variant external config accepted", name)
		}
	}
}

func TestPerforceCopyMetadataCountsOrdinaryConfigAndRefs(t *testing.T) {
	root := copyTestStore(t)
	before, err := inspectLocalObjectStore(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := localObjectCopyRequired(before, localManifest{}, 100<<30)
	if err != nil {
		t.Fatal(err)
	}
	config := "[core]\n repositoryformatversion = 0\n bare = false\n filemode = true\n[ user ]\n name = Test\n email = test@example.invalid\n[branch \"main\"]\n remote = origin\n merge = refs/heads/main\n"
	localTestWrite(t, root, ".git/config", []byte(config))
	localTestWrite(t, root, ".git/HEAD", []byte("ref: refs/heads/main\n"))
	localTestWrite(t, root, ".git/refs/heads/main", []byte(strings.Repeat("a", 40)+"\n"))
	after, err := inspectLocalObjectStore(context.Background(), root)
	if err != nil {
		t.Fatal("ordinary core/identity/branch config rejected", err)
	}
	need, err := localObjectCopyRequired(after, localManifest{}, 100<<30)
	if err != nil || need <= baseline+uint64(len(config)+61) {
		t.Fatalf("metadata costs omitted: before=%d after=%d err=%v", baseline, need, err)
	}
}

func TestPerforceCopyEmptyDirectoriesConsumePathAndSpaceBudgets(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "bb"} {
		if err := os.MkdirAll(filepath.Join(root, ".git", "objects", name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	limits := localLimits
	limits.paths = 1
	if _, err := inspectLocalObjectStoreWithLimits(context.Background(), root, limits); err == nil {
		t.Fatal("empty-directory paths bypassed scan quota")
	}
	store, err := inspectLocalObjectStore(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	need, err := localObjectCopyRequired(store, localManifest{}, 100<<30)
	if err != nil || need < (10<<30)+2*4096+6 {
		t.Fatalf("empty-directory copy costs omitted: %d %v", need, err)
	}
}
