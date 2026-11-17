package api

import (
	"context"
	"testing"

	"github.com/Patrick08T/Pangu/internal/identity"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func TestPerforceDirectoryIsVisibleOnlyToAdministrators(t *testing.T) {
	for _, admin := range []bool{false, true} {
		h := handler{access: testAccess{authorize: func(_ context.Context, _, action, _ string) error {
			if action == "admin" && !admin {
				return identity.ErrForbidden
			}
			return nil
		}}}
		out := &workspace.P4Delivery{LocalDirectory: "E:/private/workspace", ExistingChanges: []string{"default"}}
		if err := h.maskPerforceDirectory(context.Background(), out); err != nil {
			t.Fatal(err)
		}
		if (out.LocalDirectory != "") != admin {
			t.Fatal("本地路径可见性不符合管理员权限")
		}
		if len(out.ExistingChanges) != 1 {
			t.Fatal("清单信息不应隐藏")
		}
	}
}
