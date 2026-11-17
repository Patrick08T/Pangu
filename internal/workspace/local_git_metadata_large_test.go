package workspace

import (
	"context"
	"io"
	"testing"
)

type localRepeatedByte byte

func (b localRepeatedByte) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

func TestLocalGitLargeMetadataRemainsRawData(t *testing.T) {
	cases := []struct {
		name, file string
		size       int64
		fill       byte
	}{
		{"modules-large", ".gitmodules", 513 << 20, 0},
		{"attributes-large", ".gitattributes", 101 << 20, 0},
		{"attributes-line", ".gitattributes", 1 << 20, 'x'},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repository, parent := localTestRepository(t)
			data := io.LimitReader(localRepeatedByte(tc.fill), tc.size)
			commit := localTestMetadataFile(t, repository, parent, tc.file, tc.size, data)
			if _, err := git(context.Background(), repository, localObjectCheckArgs(commit)...); err != nil {
				t.Fatalf("large P4 metadata bytes rejected: %v", err)
			}
		})
	}
}
