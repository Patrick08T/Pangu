package perforce

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRecordStreamAcceptsObjectsAcrossEveryChunkBoundary(t *testing.T) {
	input := " \r\n{\"clientFile\":\"a\\\"{中}\\\\x\",\"nested\":{\"list\":[1,2]}}\r\n\t{\"clientFile\":\"z\"} "
	for chunk := 1; chunk <= len(input); chunk++ {
		var got []string
		stream := recordStream{recordLimit: 1024, totalLimit: 4096, consume: func(r Record) error { got = append(got, r["clientFile"]); return nil }}
		for start := 0; start < len(input); start += chunk {
			end := min(start+chunk, len(input))
			if _, err := stream.Write([]byte(input[start:end])); err != nil {
				t.Fatal(err)
			}
		}
		if err := stream.finish(); err != nil || !slices.Equal(got, []string{"a\"{中}\\x", "z"}) {
			t.Fatalf("chunk=%d got=%v err=%v", chunk, got, err)
		}
	}
}

func TestRecordStreamFailsClosedOnInvalidAndLateErrors(t *testing.T) {
	for _, tail := range []string{`null`, `[]`, `true`, `{"a":`, `{"a":]}`, `{"a":"unterminated}`, `{"severity":3,"data":"late"}`, `{"Root":"\ufffd"}`, "garbage"} {
		t.Run(tail, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stream := recordStream{recordLimit: 1024, totalLimit: 4096, cancel: cancel, consume: func(Record) error { return nil }}
			_, _ = stream.Write([]byte(`{"clientFile":"ok"}` + tail))
			if stream.finish() == nil || ctx.Err() == nil {
				t.Fatal("accepted malformed suffix or failed to cancel")
			}
		})
	}
}

func TestRecordStreamBudgetBoundariesAndFirstCause(t *testing.T) {
	for _, tc := range []struct {
		name          string
		record, total int
		input         string
		fail          bool
	}{
		{"record exact", 2, 10, `{}`, false}, {"record over", 1, 10, `{}`, true},
		{"command exact", 2, 4, " {} ", false}, {"command whitespace over", 2, 3, " {} ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			stream := recordStream{recordLimit: tc.record, totalLimit: tc.total, cancel: cancel, consume: func(Record) error { return nil }}
			_, err := stream.Write([]byte(tc.input))
			if err == nil {
				err = stream.finish()
			}
			if (err != nil) != tc.fail || (ctx.Err() != nil) != tc.fail {
				t.Fatalf("err=%v canceled=%v", err, ctx.Err())
			}
		})
	}
	cause := errors.New("collector first cause")
	stream := recordStream{recordLimit: 10, totalLimit: 100, consume: func(Record) error { return cause }}
	_, first := stream.Write([]byte(`{}`))
	_, second := stream.Write([]byte(strings.Repeat("x", 101)))
	if !errors.Is(first, cause) || !errors.Is(second, cause) || !errors.Is(stream.finish(), cause) {
		t.Fatal("lost first cause")
	}
}

func TestRecordStreamBudgetsAccumulateAcrossWrites(t *testing.T) {
	stream := recordStream{recordLimit: 2, totalLimit: 4, consume: func(Record) error { return nil }}
	for i := 0; i < 2; i++ {
		if _, err := stream.Write([]byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stream.Write([]byte(` `)); err == nil {
		t.Fatal("total output budget reset between writes")
	}
	stream = recordStream{recordLimit: 2, totalLimit: 100, consume: func(Record) error { return nil }}
	if _, err := stream.Write([]byte(`{"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte(`a`)); err == nil {
		t.Fatal("record budget reset between writes")
	}
}

func TestLocalFilesBudgetFailuresReturnNoPartialResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits localFileLimits
		fail   bool
	}{
		{"record", localFileLimits{record: 10, command: 4096, files: 10, paths: 100}, true},
		{"command", localFileLimits{record: 1024, command: 30, files: 10, paths: 100}, true},
		{"unique files", localFileLimits{record: 1024, command: 4096, files: 1, paths: 100}, true},
		{"path bytes", localFileLimits{record: 1024, command: 4096, files: 10, paths: 9}, true},
		{"exact unique/path budgets including duplicates", localFileLimits{record: 1024, command: 4096, files: 2, paths: 10}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fakeSnapshot(t, "pair")
			got, err := s.localFiles(t.Context(), tc.limits)
			if tc.fail {
				if err == nil || got != nil {
					t.Fatalf("partial list escaped: got=%v err=%v", got, err)
				}
			} else if err != nil || !slices.Equal(got, []string{"a.txt", "z.txt"}) {
				t.Fatalf("boundary rejected: got=%v err=%v", got, err)
			}
		})
	}
}
