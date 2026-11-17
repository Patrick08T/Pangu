package evaluation

import (
	"context"
	"os"
	"testing"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/google/uuid"
)

// The explicit baseline-only switch is for the initial stub RED against a
// separately prepared <=0055 test DB. GREEN must use the normal Migrate path.
func experimentDBScope(t *testing.T) (string, string, string) {
	t.Helper()
	if os.Getenv("PANGU_EVALUATION_BASE_SCHEMA_ONLY") != "1" {
		ns, project, _, user := evaluationFixture(t)
		return ns, project, user
	}
	if os.Getenv("PANGU_TEST_DATABASE_URL") == "" {
		t.Skip("需要独立测试数据库")
	}
	t.Setenv("PANGU_DATABASE_URL", os.Getenv("PANGU_TEST_DATABASE_URL"))
	ctx := context.Background()
	conn, err := database.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ns, project, user := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.projects(vcs,id,namespace,name,source,branch,worker_id) VALUES('git',$1,$2,'评测试点','unused','main','test')`, project, ns); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pangu.users(id,namespace,app_id,tenant_key,open_id,is_admin) VALUES($1,$2,'test','test',$3,true)`, user, ns, user); err != nil {
		t.Fatal(err)
	}
	return ns, project, user
}
