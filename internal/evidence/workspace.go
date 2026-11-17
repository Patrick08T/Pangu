package evidence

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func (b *Bundle) Materialize(directory string) (*workflow.EvidenceReference, error) {
	gitDir := filepath.Join(directory, ".git")
	resolved, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(gitDir)) {
		return nil, errors.New("Git 元数据目录不允许链接")
	}
	root := filepath.Join(gitDir, "pangu-evidence")
	if err = os.Mkdir(root, 0700); err != nil {
		return nil, err
	}
	manifest := b.Manifest()
	for _, file := range b.Files {
		if !validFile(file) {
			return nil, errors.New("证据路径或哈希无效")
		}
		if err = writeExclusive(root, file.Path, []byte(file.Body)); err != nil {
			return nil, err
		}
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = writeExclusive(root, "manifest.json", raw); err != nil {
		return nil, err
	}
	return &workflow.EvidenceReference{ManifestPath: ".git/pangu-evidence/manifest.json", SHA256: workflow.ReportHash(string(raw)), RunID: b.RunID, FileCount: len(b.Files)}, nil
}

func (b *Bundle) Manifest() Bundle {
	manifest := *b
	manifest.Files = make([]File, len(b.Files))
	for i, file := range b.Files {
		manifest.Files[i] = file
		manifest.Files[i].Body = ""
	}
	return manifest
}

func validFile(file File) bool {
	return fs.ValidPath(file.Path) && !strings.ContainsAny(file.Path, ":\\") && file.Path != "manifest.json" && workflow.ReportHash(file.Body) == file.SHA256 && len(file.Body) == file.Bytes
}

func writeExclusive(root, path string, data []byte) error {
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	return errors.Join(writeErr, closeErr)
}

func Verify(directory string, ref *workflow.EvidenceReference) error {
	if ref == nil || ref.ManifestPath != ".git/pangu-evidence/manifest.json" {
		return errors.New("证据索引引用无效")
	}
	root := filepath.Join(directory, ".git", "pangu-evidence")
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || workflow.ReportHash(string(raw)) != ref.SHA256 {
		return errors.New("平台证据索引被修改")
	}
	var manifest Bundle
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	for _, file := range manifest.Files {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil || workflow.ReportHash(string(body)) != file.SHA256 {
			return errors.New("平台原始证据被修改")
		}
	}
	return nil
}
