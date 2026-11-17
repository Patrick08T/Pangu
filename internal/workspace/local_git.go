package workspace

import (
	"io/fs"
	"time"
)

type localManifest struct {
	Files      []localFileInfo
	TotalBytes int64
	PathBytes  int64
	Missing    int
	Largest    int64
	root       string
	identity   fs.FileInfo
}

type localFileInfo struct {
	Name     string
	Size     int64
	Mode     fs.FileMode
	ModTime  time.Time
	identity fs.FileInfo
}
