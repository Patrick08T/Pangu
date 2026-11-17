package perforce

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func inventoryStageWait(stage, root string) {
	if os.Getenv("PANGU_TEST_P4_DELAY") == stage {
		time.Sleep(300 * time.Millisecond)
	}
	if os.Getenv("PANGU_TEST_P4_GATE") != stage {
		return
	}
	if os.Getenv("P4IGNORE") != "retain-ignore" {
		os.Exit(9)
	}
	_ = os.WriteFile(filepath.Join(root, "stage.ready"), nil, 0600)
	for i := 0; i < 500; i++ {
		if _, err := os.Stat(filepath.Join(root, "stage.release")); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(9)
}

func TestLocalInventoryReportsCompleteUnionAndExcludedRecordCount(t *testing.T) {
	s := inventorySnapshot(t, []string{"//daily/项目/have.txt", "//daily/项目/repo/.git/config"},
		[]string{"//daily/项目/open.txt", "//daily/项目/have.txt"}, []string{"//daily/项目/new.txt", "//daily/项目/repo/.git/config"})
	inventoryFile(t, s, "repo/.git/config", "private")
	got, err := s.LocalInventory(t.Context())
	if err != nil || !slices.Equal(got.Files, []string{"have.txt", "new.txt", "open.txt"}) || got.PathBytes != 23 || got.ExcludedMetadata != 2 {
		t.Fatalf("incomplete inventory statistics: %+v %v", got, err)
	}
	if len(got.Stages) != 4 {
		t.Fatalf("missing stages: %+v", got.Stages)
	}
	for i, want := range []struct {
		name    string
		records int
	}{{"info", 1}, {"fstat -Rh", 2}, {"fstat -Ro", 2}, {"reconcile", 2}} {
		stage := got.Stages[i]
		if stage.Name != want.name || stage.Records != want.records || stage.Duration <= 0 {
			t.Fatalf("wrong stage %d: %+v", i, stage)
		}
	}
	data, err := os.ReadFile(filepath.Join(s.LocalRoot, "commands"))
	if err != nil || string(data) != "info\nrh\nro\nreconcile\n" {
		t.Fatalf("info not shared: %q %v", data, err)
	}
}

func TestLocalInventoryFreezesExecutableAndEnvironmentForOneInvocation(t *testing.T) {
	s := inventorySnapshot(t, []string{"//daily/项目/a"}, []string{"//daily/项目/b"}, []string{"//daily/项目/c"})
	t.Setenv("PANGU_TEST_P4_GATE", "rh")
	t.Setenv("P4IGNORE", "retain-ignore")
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	type result struct {
		files []string
		err   error
	}
	done := make(chan result, 1)
	go func() { files, err := s.LocalFiles(ctx); done <- result{files, err} }()
	ready := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(filepath.Join(s.LocalRoot, "stage.ready")); err == nil {
			ready = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !ready {
		cancel()
		<-done
		t.Fatal("child never reached inventory gate")
	}
	t.Setenv("PANGU_P4_BINARY", filepath.Join(s.LocalRoot, "missing.exe"))
	t.Setenv("P4IGNORE", "changed-ignore")
	if err := os.WriteFile(filepath.Join(s.LocalRoot, "stage.release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || !slices.Equal(got.files, []string{"a", "b", "c"}) {
		t.Fatalf("invocation drifted after info: %v %v", got.files, got.err)
	}
}

func TestLocalInventoryStageBudgetsTerminateSlowZeroOutput(t *testing.T) {
	for _, stage := range []string{"info", "rh", "ro", "reconcile", "total"} {
		t.Run(stage, func(t *testing.T) {
			s := inventorySnapshot(t, []string{"//daily/项目/a"})
			delay := stage
			if stage == "total" {
				delay = "reconcile"
			}
			t.Setenv("PANGU_TEST_P4_DELAY", delay)
			budgets := localInventoryBudgets{time.Second, time.Second, time.Second, 2 * time.Second}
			switch stage {
			case "info":
				budgets.info = 50 * time.Millisecond
			case "rh", "ro":
				budgets.fstat = 50 * time.Millisecond
			case "reconcile":
				budgets.reconcile = 50 * time.Millisecond
			case "total":
				budgets.total = 100 * time.Millisecond
			}
			got, err := s.localInventory(t.Context(), localFileLimits{1 << 20, 512 << 20, 2_000_000, 256 << 20}, budgets)
			if !errors.Is(err, context.DeadlineExceeded) || got.Files != nil {
				t.Fatalf("unbounded %s stage returned partial/success: %+v %v", stage, got, err)
			}
		})
	}
}

func TestLocalInventorySlowReconcileIsNotMistakenForEmptyOrHung(t *testing.T) {
	s := inventorySnapshot(t, []string{"//daily/项目/a"}, nil, []string{"//daily/项目/discovered"})
	t.Setenv("PANGU_TEST_P4_DELAY", "reconcile")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	got, err := s.LocalFiles(ctx)
	if err != nil || !slices.Equal(got, []string{"a", "discovered"}) {
		t.Fatalf("lost delayed reconciliation: %v %v", got, err)
	}
}

func TestLocalInventoryDoesNotReuseInfoAcrossInvocations(t *testing.T) {
	s := inventorySnapshot(t, []string{"//daily/项目/a"})
	if _, err := s.LocalFiles(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PANGU_TEST_P4_MODE", "ascii-server")
	got, err := s.LocalFiles(t.Context())
	if got != nil || err == nil || !strings.Contains(err.Error(), "ASCII") {
		t.Fatalf("stale Unicode capability reused: %v %v", got, err)
	}
}
