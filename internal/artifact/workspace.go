package artifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

var ErrBlocked = errors.New("产物包含不可导出的路径、凭据或超限文件")
var ErrNotFound = errors.New("产物不存在或快照已更新，请刷新清单")

type File struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Change  string `json:"change"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256,omitempty"`
	Binary  bool   `json:"binary"`
	Blocked string `json:"blocked,omitempty"`
}

type Manifest struct {
	RunID      string `json:"run_id"`
	BaseCommit string `json:"base_commit"`
	HeadCommit string `json:"head_commit"`
	Files      []File `json:"files"`
	TotalBytes int64  `json:"total_bytes"`
}

func List(ctx context.Context, run string) (*Manifest, error) {
	release, err := workspace.Lease(ctx, run)
	if err != nil {
		return nil, err
	}
	defer release()
	set, err := workspace.Files(ctx, run)
	if err != nil {
		return nil, err
	}
	out := &Manifest{RunID: run, BaseCommit: set.Base, HeadCommit: set.Head, Files: []File{}}
	reports, err := research.Reports(ctx, run)
	if err != nil {
		return nil, err
	}
	out.TotalBytes, err = artifactBudget(set, reports)
	if err != nil {
		return nil, err
	}
	for _, change := range set.Changes {
		file, _, err := inspect(ctx, set, change)
		if err != nil {
			return nil, err
		}
		out.Files = append(out.Files, file)
	}
	for _, record := range reports {
		out.Files = append(out.Files, reportFile(record))
	}
	return out, nil
}

func inspect(ctx context.Context, set *workspace.Fileset, change workspace.Change) (File, []byte, error) {
	id := sha256.Sum256([]byte(set.Base + ":" + set.Head + ":" + change.Path))
	file := File{ID: hex.EncodeToString(id[:16]), Path: change.Path, Change: change.Status, Size: change.Size}
	file.Blocked = exportable(change.Path, change.Mode, nil, nil)
	if file.Blocked == "" && change.OldMode == "120000" {
		file.Blocked = "symlink"
	}
	if change.Size > 10*1024*1024 || change.OldSize > 10*1024*1024 {
		file.Blocked = "too_large"
	}
	if file.Blocked != "" {
		return file, nil, nil
	}
	var data, old []byte
	var err error
	if change.Mode != "000000" {
		data, err = set.Blob(ctx, change.Object)
	}
	if err == nil && change.OldMode != "000000" {
		old, err = set.Blob(ctx, change.OldObject)
	}
	if err != nil {
		return file, nil, err
	}
	file.Blocked = exportable(change.Path, change.Mode, data, old)
	file.Binary = !utf8.Valid(data) || bytes.ContainsRune(data, 0) || !utf8.Valid(old) || bytes.ContainsRune(old, 0)
	if change.Mode != "000000" {
		hash := sha256.Sum256(data)
		file.SHA256 = hex.EncodeToString(hash[:])
	}
	return file, data, nil
}

func exportable(name, mode string, data, old []byte) string {
	if !utf8.ValidString(name) || name == "." || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, ":\\\x00") || name == ".." || strings.HasPrefix(name, "../") {
		return "unsafe_path"
	}
	for _, part := range strings.Split(strings.ToLower(name), "/") {
		if part == ".git" || part == ".pangu" || part == ".aws" || strings.HasPrefix(part, ".env") || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || part == "credentials" || part == "id_rsa" || part == "id_ed25519" {
			return "sensitive"
		}
	}
	if mode != "100644" && mode != "100755" && mode != "000000" {
		return "symlink"
	}
	for _, value := range [][]byte{data, old} {
		if diagnostic.Sensitive(value) {
			return "sensitive"
		}
	}
	return ""
}

func Read(ctx context.Context, run, id string, diff bool) (File, []byte, bool, error) {
	release, leaseErr := workspace.Lease(ctx, run)
	if leaseErr != nil {
		return File{}, nil, false, leaseErr
	}
	defer release()
	if strings.Contains(id, "-") {
		return readReport(ctx, run, id)
	}
	set, err := workspace.Files(ctx, run)
	if err != nil {
		return File{}, nil, false, err
	}
	for _, change := range set.Changes {
		hash := sha256.Sum256([]byte(set.Base + ":" + set.Head + ":" + change.Path))
		if hex.EncodeToString(hash[:16]) != id {
			continue
		}
		file, data, err := inspect(ctx, set, change)
		if err != nil {
			return file, nil, false, err
		}
		if file.Blocked != "" {
			return file, nil, false, ErrBlocked
		}
		if !diff {
			if file.Change == "D" {
				return file, nil, false, ErrNotFound
			}
			return file, data, false, nil
		}
		if file.Binary {
			return file, nil, false, nil
		}
		data, clipped, err := set.Diff(ctx, change.Path)
		if err != nil {
			return file, nil, false, err
		}
		text, truncated := diagnostic.Text(string(data), 1024*1024)
		return file, []byte(text), truncated || clipped, nil
	}
	return File{}, nil, false, ErrNotFound
}
