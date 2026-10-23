package artifact

import (
	"testing"
)

func TestArtifactExportRejectsUnsafePathsAndCredentials(t *testing.T) {
	for _, path := range []string{"../secret", "/secret", "C:/secret", ".git/config", "ok/../../secret", "ok\\secret", ".env", "config/private.pem"} {
		if exportable(path, "100644", []byte("public"), nil) == "" {
			t.Errorf("接受不安全路径: %s", path)
		}
	}
	for _, body := range []string{`{"api_key":"secret-value"}`, "-----BEGIN PRIVATE KEY-----\nvalue", "password=secret-value"} {
		if exportable("report.txt", "100644", []byte(body), nil) == "" {
			t.Error("接受疑似凭据内容")
		}
	}
	if exportable("src/main.go", "100644", []byte("package main\n"), nil) != "" {
		t.Fatal("拒绝普通源文件")
	}
	if exportable("link", "120000", []byte("../secret"), nil) == "" {
		t.Fatal("接受符号链接")
	}
	if exportable("report.txt", "100644", []byte("public"), []byte("password=old-secret")) == "" {
		t.Fatal("差异泄露旧凭据")
	}
}
