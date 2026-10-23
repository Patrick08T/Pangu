package workflow

import (
	"testing"
	"time"
)

func TestCronPreviewUsesUTCAndFiveFutureOccurrences(t *testing.T) {
	now := time.Date(2026, 10, 23, 18, 0, 0, 0, time.FixedZone("SG", 8*3600))
	times, err := Preview("0 12 * * *", now)
	if err != nil || len(times) != 5 || times[0].Format(time.RFC3339) != "2026-10-23T12:00:00Z" {
		t.Fatalf("预览时区或次数错误: %+v %v", times, err)
	}
	if _, err := Preview("0 0 0 * * *", now); err == nil {
		t.Fatal("接受六字段表达式")
	}
	if _, err := Preview("0 0 31 2 *", now); err == nil {
		t.Fatal("接受永不触发的表达式")
	}
}
