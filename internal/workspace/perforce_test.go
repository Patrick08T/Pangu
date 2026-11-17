package workspace

import (
	"encoding/json"
	"testing"
)

func TestPerforceProjectValidatesSeparateSourceFields(t *testing.T) {
	valid := `{"name":"P4 项目","vcs":"perforce","worker_id":"test","perforce":{"port":"localhost:1666","user":"tester","client":"template","scope":"Server"}}`
	var project Project
	if err := json.Unmarshal([]byte(valid), &project); err != nil {
		t.Fatal(err)
	}
	project.Source = t.TempDir()
	if err := project.Validate(); err != nil {
		t.Fatal("有效 P4 来源被拒绝", err)
	}
}

func TestProjectRejectsUnknownVersionControl(t *testing.T) {
	var project Project
	data, _ := json.Marshal(map[string]any{"name": "错误来源", "vcs": "unknown", "source": t.TempDir(), "branch": "main", "worker_id": "test"})
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}
	if project.Validate() == nil {
		t.Fatal("接受未知来源类型")
	}
}
