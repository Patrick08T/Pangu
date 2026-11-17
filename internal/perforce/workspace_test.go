package perforce

import (
	"testing"
)

func TestRejectUnsafeServerAndScope(t *testing.T) {
	for _, field := range []struct{ port, scope string }{{"rsh:evil:1666", "Server"}, {"localhost:1666", "../outside"}, {"localhost:1666", "Server/..."}, {"localhost:1666", ".git"}, {"localhost:1666", "/root"}, {"localhost:1666", "Server@1"}} {
		if (Config{Port: field.port, Scope: field.scope, User: "tester", Client: "daily"}).Validate() == nil {
			t.Fatalf("接受非法来源 %+v", field)
		}
	}
}

func TestRemapPreservesExclusionsAndRejectsForeignClient(t *testing.T) {
	view, err := remap([]string{`//depot/... //daily/...`, `-//depot/Assets/... //daily/Assets/...`, `"//depot/space name/..." "//daily/space name/..."`}, "daily", "pangu-run")
	if err != nil || len(view) != 3 || view[1] != `-//depot/Assets/... "//pangu-run/Assets/..."` {
		t.Fatalf("排除映射丢失 %v %v", view, err)
	}
	if _, err = remap([]string{`//depot/... //someone/...`}, "daily", "pangu-run"); err == nil {
		t.Fatal("接受其他 Client 映射")
	}
}

func TestMalformedMetadataPathIsRejected(t *testing.T) {
	if _, err := parse(`{"clientFile":"//client/\ufffd.txt"}`); err == nil {
		t.Fatal("接受乱码路径")
	}
}
