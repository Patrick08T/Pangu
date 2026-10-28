package research

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSearchDelegatesOnlyAuthorizedQueryAndRegistersUnreadSources(t *testing.T) {
	in := researchInput()
	in.Input = `{"question":"调研","public_queries":["公开关键词"]}`
	calls := 0
	s, err := New(in, func(_ context.Context, query string) ([]SearchResult, error) {
		calls++
		if query != "公开关键词" {
			t.Fatal("公开查询被扩大", query)
		}
		return []SearchResult{{URL: "http://bad.example"}, {URL: "https://127.0.0.1"},
			{URL: "https://example.com/docs", Title: "官方文档"}, {URL: "https://example.com/docs"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.invoke(context.Background(), "search_public", `{"query_index":1}`); err == nil || calls != 0 {
		t.Fatal("越权查询到达搜索服务")
	}
	raw, err := s.invoke(context.Background(), "search_public", `{"query_index":0}`)
	var got []Source
	if err != nil || json.Unmarshal([]byte(raw), &got) != nil || len(got) != 1 || got[0].Read || got[0].Title != "官方文档" {
		t.Fatalf("搜索来源错误: %s %v", raw, err)
	}
	if _, ok := s.sourceText[got[0].ID]; ok {
		t.Fatal("搜索摘要被当成已读取正文")
	}
	if len(s.evidence) != 2 || s.evidence[1].ExitCode != 0 {
		t.Fatal("搜索缺少执行记录")
	}
}
