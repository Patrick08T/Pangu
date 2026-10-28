package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestReadOnlyCodexUsesNativeSandbox(t *testing.T) {
	setupCodex(t)
	input := codexInput("test")
	input.Node.ReadOnly = true
	output, err := Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Args []string `json:"args"`
	}
	if err = json.Unmarshal([]byte(output.Text), &result); err != nil {
		t.Fatal(err)
	}
	for i, arg := range result.Args {
		if arg == "--sandbox" && i+1 < len(result.Args) && result.Args[i+1] == "read-only" {
			return
		}
	}
	t.Fatal("只读节点没有设置原生 CLI 沙箱")
}
