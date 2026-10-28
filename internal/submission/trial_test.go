package submission

import (
	"errors"
	"github.com/google/uuid"
	"strconv"
	"testing"
	"time"
)

func TestTrialRequestKeyTimeWindow(t *testing.T) {
	for _, test := range []struct {
		age   time.Duration
		valid bool
	}{{0, true}, {-4 * time.Minute, true}, {-6 * time.Minute, false}, {90 * 24 * time.Hour, false}} {
		key := strconv.FormatInt(time.Now().Add(-test.age).UnixMilli(), 10) + "." + uuid.NewString()
		if err := CheckTrialKey(key); (err == nil) != test.valid {
			t.Fatalf("键时限错误: %v %v", test, err)
		}
	}
	for _, key := range []string{"", uuid.NewString(), "9." + uuid.NewString(), "9999999999999.bad"} {
		if CheckTrialKey(key) == nil {
			t.Fatal("接受错误格式")
		}
	}
	old := strconv.FormatInt(time.Now().Add(-90*24*time.Hour).UnixMilli(), 10) + "." + uuid.NewString()
	if !errors.Is(CheckTrialKey(old), ErrExpired) {
		t.Fatal("旧键未标记过期")
	}
	if PublicKey(TrialKey("external")) {
		t.Fatal("外部调用可使用内部试运行键")
	}
}
