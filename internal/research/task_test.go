package research

import "testing"

func TestDesignTaskDependenciesAndAcceptance(t *testing.T) {
	tasks := []Task{{ID: "api", Title: "API", Scope: "输入校验", Deliverable: "接口", Acceptance: "非法输入返回 400"}, {ID: "ui", Title: "界面", Scope: "提交表单", DependsOn: []string{"api"}, Deliverable: "页面", Acceptance: "可见提交状态"}}
	if _, err := taskAppendix(tasks); err != nil {
		t.Fatal(err)
	}
	tasks[0].DependsOn = []string{"ui"}
	if _, err := taskAppendix(tasks); err == nil {
		t.Fatal("接受循环任务依赖")
	}
	tasks[0].DependsOn = nil
	tasks[1].Acceptance = ""
	if _, err := taskAppendix(tasks); err == nil {
		t.Fatal("接受没有验收条件的任务")
	}
}
