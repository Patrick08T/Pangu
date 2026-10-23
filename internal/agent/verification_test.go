package agent

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"os"
	"testing"
	"time"
)

func TestVerificationUsesActualExitAndCancelsProcess(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"pass", "fail", "wait"} {
		timeout := 5 * time.Second
		if mode == "wait" {
			timeout = 200 * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		evidence, err := Verify(ctx, t.TempDir(), []string{binary, "verify-helper", mode}, "")
		cancel()
		if mode == "pass" && (err != nil || evidence.ExitCode != 0) {
			t.Fatalf("验证成功未记录: %+v %v", evidence, err)
		}
		if mode == "fail" && (err == nil || evidence.ExitCode != 7 || workflow.CanRetry(err)) {
			t.Fatalf("失败或重试分类错误: %+v %v", evidence, err)
		}
		if mode == "wait" && err == nil {
			t.Fatal("未终止超时验证")
		}
	}
}
