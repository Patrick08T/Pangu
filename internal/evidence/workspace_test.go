package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterializeProtectsFilesAndKeepsPayloadSmall(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	b := &Bundle{RunID: "run", ProjectID: "project", Files: []File{}}
	if err := b.add("run", "report", "原文", "reports/example.txt", "原始证据"); err != nil {
		t.Fatal(err)
	}
	ref, err := b.Materialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref.ManifestPath)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Bundle
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Body != "" {
		t.Fatal("索引重复包含全文")
	}
	if _, err = b.Materialize(dir); err == nil {
		t.Fatal("覆盖了已有运行期证据")
	}
	if err = Verify(dir, ref); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".git", "pangu-evidence", "run", "reports", "example.txt")
	if err = os.WriteFile(path, []byte("篡改证据"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Verify(dir, ref); err == nil {
		t.Fatal("模型修改证据后仍可成功交付")
	}
}

func TestMaterializeRejectsTraversalAndTampering(t *testing.T) {
	for _, path := range []string{"../outside.txt", "C:/outside.txt", "run/body.txt"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		b := &Bundle{Files: []File{{Path: path, Body: "被替换的正文", SHA256: "invalid"}}}
		if _, err := b.Materialize(dir); err == nil {
			t.Fatal("写入了无效证据", path)
		}
	}
}
