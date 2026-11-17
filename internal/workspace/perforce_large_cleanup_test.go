package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestP4PreparationCleanupRequiresConfirmedStop(t *testing.T) {
	cases := []struct {
		name     string
		cause    error
		retained bool
	}{
		{"success", nil, true},
		{"confirmed-cancel", context.Canceled, false},
		{"ordinary-failure", os.ErrPermission, false},
		{"unconfirmed-stop", errors.Join(context.Canceled, errLocalGitStopUnconfirmed), true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			name := "preparing-local-" + uuid.NewString()
			if err := os.Mkdir(filepath.Join(directory, name), 0700); err != nil {
				t.Fatal(err)
			}
			keep := filepath.Join(directory, "base")
			if err := os.Mkdir(keep, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			err = cleanupLocalPreparation(root, name, test.cause)
			if !errors.Is(err, test.cause) {
				t.Fatalf("original cause lost: %v", err)
			}
			_, statErr := os.Lstat(filepath.Join(directory, name))
			if test.retained != (statErr == nil) {
				t.Fatalf("staging retained=%v, want %v: %v", statErr == nil, test.retained, statErr)
			}
			if _, err := os.Stat(keep); err != nil {
				t.Fatalf("published repository altered: %v", err)
			}
		})
	}
}

func TestP4PreparationCleanupCannotTargetPublishedOrExternalPaths(t *testing.T) {
	private := t.TempDir()
	directory := filepath.Join(private, "owned")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"base", "source", "../preparing-local-" + uuid.NewString(), "preparing-local-invalid"} {
		path := filepath.Join(directory, name)
		relative, err := filepath.Rel(private, path)
		if err != nil || !filepath.IsLocal(relative) {
			t.Fatal("fixture escaped private directory")
		}
		if err := os.WriteFile(path, []byte("evidence"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cleanupLocalPreparation(root, name, os.ErrPermission); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("cause not retained: %v", err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "evidence" {
			t.Fatalf("unowned evidence changed: %q %v", data, err)
		}
	}
}
