package research

import (
	"strings"
	"testing"
)

func TestClaimsRequireActualQuotesAndSeparateConflictsFromFacts(t *testing.T) {
	session, err := New(researchInput(), nil)
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{ID: "c1", Statement: "现状依赖手工登记", Kind: "fact", Scope: "本次团队", Uncertainty: "样本仅为用户提供的背景", Evidence: []Evidence{{SourceID: "input", Quote: "现状为手工登记", SourceDate: "unknown"}}}
	if _, err = session.claims([]Claim{claim}, []string{"input"}); err != nil {
		t.Fatal(err)
	}
	claim.Evidence[0].Quote = "并不存在的原文"
	if _, err = session.claims([]Claim{claim}, []string{"input"}); err == nil {
		t.Fatal("读过来源被误当成任意结论的证据")
	}
	claim.Evidence[0].Quote = "现状为手工登记"
	claim.Conflicts = []string{"用户另一份资料称已自动化"}
	if _, err = session.claims([]Claim{claim}, []string{"input"}); err == nil {
		t.Fatal("冲突被包装为确定事实")
	}
	claim.Kind = "unknown"
	appendix, err := session.claims([]Claim{claim}, []string{"input"})
	if err != nil || !strings.Contains(appendix, "尚需人工判定") || !strings.Contains(appendix, "unknown") {
		t.Fatalf("未知或核验边界丢失: %s %v", appendix, err)
	}
}
