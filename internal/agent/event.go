package agent

import (
	"bytes"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

type codexEvents struct {
	line         []byte
	dropping     bool
	Commands     []workflow.CommandEvidence
	Truncated    bool
	size         int
	ErrorMessage string
}

func (events *codexEvents) Write(data []byte) (int, error) {
	length := len(data)
	for len(data) > 0 {
		index := bytes.IndexByte(data, '\n')
		size := len(data)
		if index >= 0 {
			size = index
		}
		if len(events.line)+size > 1024*1024 {
			events.dropping, events.Truncated = true, true
		}
		if !events.dropping {
			events.line = append(events.line, data[:size]...)
		}
		if index < 0 {
			break
		}
		if !events.dropping {
			events.record(events.line)
		}
		events.line, events.dropping = nil, false
		data = data[index+1:]
	}
	return length, nil
}

func (events *codexEvents) record(line []byte) {
	var event struct {
		Type    string
		Message string
		Error   struct{ Message string }
		Item    struct {
			ID       string
			Type     string
			Command  string
			Output   string `json:"aggregated_output"`
			ExitCode *int   `json:"exit_code"`
		}
	}
	if json.Unmarshal(line, &event) != nil {
		return
	}
	if event.Type == "turn.failed" {
		events.ErrorMessage, _ = diagnostic.Text(event.Error.Message, 4096)
	}
	if event.Type == "error" {
		events.ErrorMessage, _ = diagnostic.Text(event.Message, 4096)
	}
	if event.Type != "item.completed" || event.Item.Type != "command_execution" || event.Item.ExitCode == nil {
		return
	}
	if len(events.Commands) >= 100 {
		events.Truncated = true
		return
	}
	command, a := diagnostic.Text(event.Item.Command, 1024)
	output, b := diagnostic.Text(event.Item.Output, 4096)
	id, c := diagnostic.Text(event.Item.ID, 128)
	record := workflow.CommandEvidence{ID: id, Command: command, Output: output, ExitCode: *event.Item.ExitCode, Truncated: a || b || c}
	encoded, _ := json.Marshal(record)
	if events.size+len(encoded) > 1024*1024 {
		events.Truncated = true
		return
	}
	events.size += len(encoded)
	events.Commands = append(events.Commands, record)
}
