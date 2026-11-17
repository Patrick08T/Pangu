package workspace

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestLocalGitNULBudgetsRejectOnlyOverflow(t *testing.T) {
	cases := []struct {
		name, input string
		limits      localGitLimits
		want        int
	}{
		{"exact record", "abc\x00", localGitLimits{3, 4, 1, 3}, 1},
		{"record overflow", "abcd\x00", localGitLimits{3, 5, 1, 3}, 0},
		{"total overflow", "abc\x00", localGitLimits{3, 3, 1, 3}, 0},
		{"count overflow", "a\x00b\x00", localGitLimits{3, 4, 1, 3}, 1},
		{"path overflow", "ab\x00cd\x00", localGitLimits{3, 6, 2, 3}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			count, canceled := 0, false
			s := localNULStream{ctx: context.Background(), limits: tc.limits, cancel: func() { canceled = true }, visit: func([]byte) error { count++; return nil }}
			_, err := s.Write([]byte(tc.input))
			if count != tc.want || (err == nil) != (tc.name == "exact record") || canceled != (err != nil) {
				t.Fatalf("count=%d cancel=%v error=%v", count, canceled, err)
			}
		})
	}
}

func TestLocalGitNULSplitsAndPreservesVisitorCause(t *testing.T) {
	var got []string
	cause := errors.New("late path failure")
	s := localNULStream{ctx: context.Background(), limits: localGitLimits{4, 20, 4, 10}, cancel: func() {}, visit: func(p []byte) error {
		got = append(got, string(p))
		if len(got) == 2 {
			return cause
		}
		return nil
	}}
	for _, chunk := range []string{"ab", "c\x00", "d\x00ignored"} {
		_, _ = s.Write([]byte(chunk))
	}
	if strings.Join(got, ",") != "abc,d" || !errors.Is(s.err, cause) {
		t.Fatalf("records=%q error=%v", got, s.err)
	}
	if _, err := s.Write([]byte("anything")); !errors.Is(err, cause) {
		t.Fatalf("first cause overwritten: %v", err)
	}
}

func TestLocalGitTreePathBudgetIncludesLastRecord(t *testing.T) {
	row := "100644 blob " + strings.Repeat("a", 40) + " 0\taaa"
	check := localTreeCheck{paths: map[string]bool{}, limits: localGitLimits{100, 500, 2, 3}}
	if err := check.visit([]byte(row)); err != nil {
		t.Fatal(err)
	}
	if err := check.visit([]byte(strings.TrimSuffix(row, "aaa") + "b")); err == nil {
		t.Fatal("last path exceeded budget undetected")
	}
}

func TestLocalGitCapacityUsesCeilingReserveAndRejectsOverflow(t *testing.T) {
	m := localManifest{Files: make([]localFileInfo, 2), TotalBytes: 5, PathBytes: 7}
	for _, tc := range []struct{ total, want uint64 }{{100 << 30, (10 << 30) + 8218}, {300 << 30, (15 << 30) + 8218}} {
		got, err := localRequiredSpace(m, tc.total)
		if err != nil || got != tc.want {
			t.Fatalf("capacity=%d want=%d error=%v", got, tc.want, err)
		}
	}
	m.TotalBytes = 1
	if got, err := localRequiredSpace(m, 100<<30); err != nil || got != (10<<30)+8209 {
		t.Fatalf("fractional byte rounded down: %d %v", got, err)
	}
	for _, size := range []int64{-1, math.MaxInt64} {
		m.TotalBytes = size
		if _, err := localRequiredSpace(m, 100<<30); err == nil {
			t.Fatalf("invalid size accepted: %d", size)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := checkLocalCaptureCapacity(ctx, t.TempDir(), localManifest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("capacity ignored cancellation: %v", err)
	}
}
