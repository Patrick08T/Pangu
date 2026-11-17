package perforce

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func fakeInventory(args []string) bool {
	if os.Getenv("PANGU_TEST_P4_INVENTORY") != "1" {
		return false
	}
	stage := "info"
	if !slices.Contains(args, "info") {
		if err := fakeP4Contract(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(7)
		}
		stage = "reconcile"
		if slices.Contains(args, "-Rh") {
			stage = "rh"
		}
		if slices.Contains(args, "-Ro") {
			stage = "ro"
		}
	}
	root := os.Getenv("PANGU_TEST_P4_ROOT")
	log, err := os.OpenFile(filepath.Join(root, "commands"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(8)
	}
	fmt.Fprintln(log, stage)
	log.Close()
	inventoryStageWait(stage, root)
	if stage == "info" {
		fakeP4Info(args)
		return true
	}
	f, err := os.Open(filepath.Join(root, "inventory-"+stage))
	if err != nil {
		os.Exit(8)
	}
	defer f.Close()
	if _, err := io.Copy(os.Stdout, f); err != nil {
		os.Exit(8)
	}
	return true
}

func inventorySnapshot(t *testing.T, stages ...[]string) Snapshot {
	t.Helper()
	s := fakeSnapshot(t, "inventory")
	t.Setenv("PANGU_TEST_P4_INVENTORY", "1")
	if err := os.MkdirAll(filepath.Join(s.LocalRoot, "项目"), 0700); err != nil {
		t.Fatal(err)
	}
	for i, stage := range []string{"rh", "ro", "reconcile"} {
		var data []byte
		if i < len(stages) {
			for _, path := range stages[i] {
				row, err := json.Marshal(Record{"clientFile": path})
				if err != nil {
					t.Fatal(err)
				}
				data = append(data, row...)
			}
		}
		if err := os.WriteFile(filepath.Join(s.LocalRoot, "inventory-"+stage), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func inventoryFile(t *testing.T, s Snapshot, path, body string) string {
	t.Helper()
	name := filepath.Join(s.LocalRoot, "项目", filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestLocalInventoryExcludesNestedMetadataWithoutChangingOrdinaryContent(t *testing.T) {
	s := inventorySnapshot(t, []string{"//daily/项目/engine/.git/config", "//daily/项目/engine/code.go", "//daily/项目/sub/.git"},
		[]string{"//daily/项目/sub/code.go", "//daily/项目/.gitignore"}, []string{"//daily/项目/.gitattributes", "//daily/项目/.gitmodules"})
	bodies := map[string]string{"engine/.git/config": "private configuration", "engine/code.go": "engine", "sub/.git": "gitdir: Z:/must-not-read", "sub/code.go": "sub", ".gitignore": "ignore", ".gitattributes": "attributes", ".gitmodules": "modules"}
	for name, body := range bodies {
		inventoryFile(t, s, name, body)
	}
	got, err := s.LocalFiles(context.Background())
	want := []string{".gitattributes", ".gitignore", ".gitmodules", "engine/code.go", "sub/code.go"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("nested metadata poisoned ordinary inventory: got=%v err=%v", got, err)
	}
	for name, body := range bodies {
		data, err := os.ReadFile(filepath.Join(s.LocalRoot, "项目", filepath.FromSlash(name)))
		if err != nil || string(data) != body {
			t.Fatalf("source changed %s: %q %v", name, data, err)
		}
	}
	for _, path := range []string{"//daily/项目/engine/.git/config", "//daily/项目/sub/.git"} {
		if _, err := s.localName(path); err == nil {
			t.Fatalf("delivery accepted metadata: %s", path)
		}
	}
}

func TestLocalInventoryRejectsRootMetadataEvenWhenP4OmitsIt(t *testing.T) {
	for _, marker := range []string{".git", ".GiT"} {
		t.Run(marker, func(t *testing.T) {
			s := inventorySnapshot(t, []string{"//daily/项目/ok.txt"})
			inventoryFile(t, s, marker, "gitdir: Z:/must-not-read")
			got, err := s.LocalFiles(t.Context())
			if err == nil || got != nil {
				t.Fatalf("ignored root metadata accepted: %v %v", got, err)
			}
		})
	}
}

func TestLocalInventoryValidatesEntireExcludedPathBeforeClassification(t *testing.T) {
	for _, path := range []string{"//daily/项目/repo/.git/../safe", "//daily/项目/repo/.git/a...b", "//daily/项目/repo/.git/%zz", "//daily/项目/repo/.git/%ff", "//daily/项目/repo/.git/a.", "//daily/项目/repo/.git/a:stream", "//other/项目/repo/.git/config"} {
		t.Run(path, func(t *testing.T) {
			s := inventorySnapshot(t, []string{path})
			inventoryFile(t, s, "repo/.git/config", "private")
			got, err := s.LocalFiles(t.Context())
			if err == nil || got != nil {
				t.Fatalf("metadata hid unsafe suffix: %s %v %v", path, got, err)
			}
		})
	}
}
