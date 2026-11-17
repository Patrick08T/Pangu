package workspace

import (
	"context"
	"testing"
)

func TestLocalGitKeepsOrdinaryDevicePrefixFilenames(t *testing.T) {
	root := t.TempDir()
	names := []string{"COM12.txt", "LPT23.log", "company.txt", "nested/.gitignore", "nested/.gitattributes", "nested/.gitmodules"}
	for _, name := range names {
		localTestWrite(t, root, name, []byte("raw"))
	}
	m, err := inspectLocalFiles(context.Background(), root, names)
	if err != nil || len(m.Files) != len(names) {
		t.Fatalf("ordinary tracked content rejected: files=%d error=%v", len(m.Files), err)
	}
}

func TestLocalGitVolumeReserveRoundsUpFractionalBytes(t *testing.T) {
	total := uint64(400<<30) + 1
	need, err := localRequiredSpace(localManifest{}, total)
	if err != nil || need != uint64(20<<30)+1 {
		t.Fatalf("5 percent reserve rounded down: %d %v", need, err)
	}
}
