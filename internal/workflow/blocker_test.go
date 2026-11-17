package workflow

import "testing"

func TestEvidenceBlockersRequireOwnerAndResolution(t *testing.T) {
	for _, body := range []string{
		`{"evidence_blockers":[{"id":"QA1","reason":"缺少审批"}]}`,
		`{"evidence_blockers":[{"id":"QA1","owner":"平台","reason":"缺少审批","resolution_condition":"提供消费记录"},{"id":"QA1","owner":"平台","reason":"重复","resolution_condition":"补齐"}]}`,
	} {
		if _, err := ParseEvidenceBlockers(body); err == nil {
			t.Fatal("模糊阻塞被接受", body)
		}
	}
	items, err := ParseEvidenceBlockers(`{"evidence_blockers":[{"id":"QA1","owner":"平台管理员","reason":"缺少审批消费记录","resolution_condition":"核对真实消费时间"}]}`)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	if err = EvidenceReview(items).Validate(); err != nil {
		t.Fatal(err)
	}
}
