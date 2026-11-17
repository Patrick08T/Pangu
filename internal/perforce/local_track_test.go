package perforce

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type localTrackCommand struct {
	Name, Change string
	Files        []string
}

func fakeTrack(args []string) bool {
	if os.Getenv("PANGU_TEST_P4_TRACK") != "1" {
		return false
	}
	if slices.Contains(args, "info") {
		fakeP4Info(args)
		return true
	}
	command := localTrackCommand{}
	for i, arg := range args {
		if arg == "opened" || arg == "reconcile" || arg == "shelve" || arg == "reopen" {
			command.Name = arg
			args = args[i+1:]
			break
		}
	}
	if len(args) >= 2 && args[0] == "-c" {
		command.Change = args[1]
		args = args[2:]
	}
	command.Files = append(command.Files, args...)
	input, _ := io.ReadAll(os.Stdin)
	if len(input) > 0 {
		command.Files = append(command.Files, strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")...)
	}
	root := os.Getenv("PANGU_TEST_P4_ROOT")
	log, err := os.OpenFile(filepath.Join(root, "tracking"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(8)
	}
	_ = json.NewEncoder(log).Encode(command)
	log.Close()
	if command.Name == "opened" {
		file, err := os.Open(filepath.Join(root, "opened"))
		if err != nil {
			os.Exit(8)
		}
		defer file.Close()
		_, _ = io.Copy(os.Stdout, file)
		return true
	}
	if command.Name != "reconcile" {
		os.Exit(9)
	}
	fmt.Print(`{"data":"reconciled"}`)
	return true
}

func trackSnapshot(t *testing.T, rows []Record) Snapshot {
	t.Helper()
	s := fakeSnapshot(t, "track")
	s.WorkChange = 99
	t.Setenv("PANGU_TEST_P4_TRACK", "1")
	file, err := os.Create(filepath.Join(s.LocalRoot, "opened"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, row := range rows {
		if err := json.NewEncoder(file).Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func trackingCommands(t *testing.T, s Snapshot) []localTrackCommand {
	t.Helper()
	file, err := os.Open(filepath.Join(s.LocalRoot, "tracking"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var result []localTrackCommand
	decoder := json.NewDecoder(file)
	for {
		var command localTrackCommand
		err := decoder.Decode(&command)
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, command)
	}
}

func TestTrackLocalBatchesWithoutMovingOriginalChangelists(t *testing.T) {
	var paths []string
	var rows []Record
	want := map[string]string{}
	for i := 0; i < 600; i++ {
		path := fmt.Sprintf("file-%04d.txt", i)
		paths = append(paths, path)
		change := []string{"11", "default", "99"}[i%3]
		want["//daily/项目/"+path] = change
		rows = append(rows, Record{"clientFile": "//daily/项目/" + path, "change": change})
	}
	s := trackSnapshot(t, rows)
	paths = append(paths, "fresh.txt")
	want["//daily/项目/fresh.txt"] = "99"
	got, err := s.TrackLocal(t.Context(), paths)
	if err != nil || !slices.Equal(got, []string{"11", "default"}) {
		t.Fatalf("lost original ownership: %v %v", got, err)
	}
	commands := trackingCommands(t, s)
	writes := 0
	seen := map[string]bool{}
	for _, command := range commands {
		if command.Name == "opened" {
			continue
		}
		writes++
		if command.Name != "reconcile" || len(command.Files) == 0 || len(command.Files) > 128 {
			t.Fatalf("unbounded/wrong write: %+v", command)
		}
		for _, file := range command.Files {
			if seen[file] || want[file] != command.Change {
				t.Fatalf("changed ownership or duplicate %s: %+v", file, command)
			}
			seen[file] = true
		}
	}
	if len(seen) != 601 || writes > 9 {
		t.Fatalf("not bounded batching: seen=%d writes=%d", len(seen), writes)
	}
}

func TestTrackLocalStreamsOpenedBeyondOrdinaryCommandCap(t *testing.T) {
	s := trackSnapshot(t, nil)
	file, err := os.Create(filepath.Join(s.LocalRoot, "opened"))
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	for i := 0; i < 30000; i++ {
		fmt.Fprintf(writer, "{\"clientFile\":\"//daily/项目/existing\",\"change\":\"11\",\"padding\":\"%s\"}\n", strings.Repeat("x", 300))
	}
	writer.Flush()
	file.Close()
	got, err := s.TrackLocal(t.Context(), []string{"existing", "fresh"})
	if err != nil || !slices.Equal(got, []string{"11"}) {
		t.Fatalf("opened truncated or ownership lost: %v %v", got, err)
	}
	commands := trackingCommands(t, s)
	if len(commands) != 3 || commands[1].Change != "11" || commands[2].Change != "99" {
		t.Fatalf("wrong streamed ownership: %+v", commands)
	}
}

func TestTrackLocalEmptyChangesDoNotStartP4(t *testing.T) {
	s := trackSnapshot(t, nil)
	t.Setenv("PANGU_P4_BINARY", filepath.Join(s.LocalRoot, "missing.exe"))
	got, err := s.TrackLocal(t.Context(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty changes started P4: %v %v", got, err)
	}
}

func TestShelveLocalStillRejectsNestedMetadata(t *testing.T) {
	s := trackSnapshot(t, []Record{{"clientFile": "//daily/项目/repo/.git/config", "change": "99"}})
	ok, err := s.ShelveLocal(t.Context(), []string{"repo/.git/config"})
	if ok || err == nil {
		t.Fatalf("delivery allowed excluded metadata: %v %v", ok, err)
	}
	commands := trackingCommands(t, s)
	if len(commands) != 1 || commands[0].Name != "opened" {
		t.Fatalf("shelved metadata: %+v", commands)
	}
}
