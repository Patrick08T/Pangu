package perforce

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackLocalValidatesAllPathsBeforeWriting(t *testing.T) {
	for _, path := range []string{"repo/.git/config", "../escape", "repo/../hidden", "file:stream", "bad\nfile"} {
		t.Run(path, func(t *testing.T) {
			s := trackSnapshot(t, nil)
			got, err := s.TrackLocal(t.Context(), []string{"safe", path})
			if err == nil || got != nil {
				t.Fatalf("unsafe path accepted: %s %v %v", path, got, err)
			}
			data, readErr := os.ReadFile(filepath.Join(s.LocalRoot, "tracking"))
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if strings.Contains(string(data), "reconcile") {
				t.Fatalf("wrote before validating complete path set: %s", data)
			}
		})
	}
}

func TestTrackLocalBatchesRespectArgumentBytes(t *testing.T) {
	s := trackSnapshot(t, nil)
	var paths []string
	for i := 0; i < 100; i++ {
		paths = append(paths, strings.Repeat("a", 200)+"/"+fmt.Sprintf("%03d", i)+strings.Repeat("b", 200))
	}
	if _, err := s.TrackLocal(t.Context(), paths); err != nil {
		t.Fatal(err)
	}
	commands := trackingCommands(t, s)
	if len(commands) > 6 {
		t.Fatalf("still per-file tracking: %d commands", len(commands))
	}
	for _, command := range commands {
		if command.Name != "reconcile" {
			continue
		}
		bytes := 0
		for _, file := range command.Files {
			bytes += len(file) + 1
		}
		if bytes > 16<<10 {
			t.Fatalf("unbounded argument bytes: %d", bytes)
		}
	}
}

func TestTrackLocalLateOpenedErrorPreventsAllWrites(t *testing.T) {
	s := trackSnapshot(t, []Record{{"clientFile": "//daily/项目/a", "change": "11"}, {"severity": "3", "data": "late ownership failure"}})
	got, err := s.TrackLocal(t.Context(), []string{"a", "fresh"})
	if got != nil || err == nil || !strings.Contains(err.Error(), "late ownership failure") {
		t.Fatalf("lost late ownership error: %v %v", got, err)
	}
	commands := trackingCommands(t, s)
	if len(commands) != 1 || commands[0].Name != "opened" {
		t.Fatalf("write before complete ownership: %+v", commands)
	}
}

func TestTrackLocalOversizedPathIsRejectedBeforeAnyWrite(t *testing.T) {
	s := trackSnapshot(t, nil)
	paths := []string{strings.Repeat("z", 1<<20)}
	for i := 0; i < 128; i++ {
		paths = append(paths, fmt.Sprintf("a-%03d", i))
	}
	got, err := s.TrackLocal(t.Context(), paths)
	if err == nil || got != nil {
		t.Fatalf("oversized path accepted: %v %v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(s.LocalRoot, "tracking"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "reconcile") {
		t.Fatalf("wrote before complete path budget validation: %s", data)
	}
}
