package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLocalGitPublicationFailureRemovesOnlyOwnedReferences(t *testing.T) {
	repository, parent := localTestRepository(t)
	temporary, final := "refs/pangu-import/test", "refs/heads/local-test"
	for _, ref := range []string{temporary, final} {
		if _, err := git(context.Background(), repository, "update-ref", ref, parent); err != nil {
			t.Fatal(err)
		}
	}
	cause := errors.New("publication command canceled after write")
	if err := cleanupLocalImport(repository, temporary, final, parent, cause); !errors.Is(err, cause) {
		t.Fatalf("original cause lost: %v", err)
	}
	assertLocalNoPublishedImport(t, repository, parent)
	if _, err := git(context.Background(), repository, "update-ref", final, parent); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{strings.Repeat("0", 40), strings.Repeat("f", 40)} {
		if err := cleanupLocalImport(repository, temporary, final, expected, cause); err == nil {
			t.Fatal("cleanup ignored mismatched owned identity")
		}
		got, err := git(context.Background(), repository, "rev-parse", final)
		if err != nil || got != parent {
			t.Fatalf("cleanup removed somebody else's reference: %q %v", got, err)
		}
	}
}
