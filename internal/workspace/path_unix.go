//go:build !windows

package workspace

import "path/filepath"

func existingPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
