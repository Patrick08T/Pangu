package agent

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/usage"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

type codexEvents struct {
	Context      context.Context
	Cancel       context.CancelFunc
	Prefix       string
	TraceErr     error
	Search       *codexSearch
	Usage        *usage.Tokens
	line         []byte
	dropping     bool
	Commands     []workflow.CommandEvidence
	Truncated    bool
	size         int
	ErrorMessage string
	ErrorStatus  int
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
	return length, events.TraceErr
}

func (events *codexEvents) record(line []byte) {
	events.trace(line)
	if events.Search != nil {
		events.Search.record(line)
	}
	var event struct {
		Usage *struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
			Cached int64 `json:"cached_input_tokens"`
		} `json:"usage"`
		Type    string
		Message string
		Status  int
		Error   struct{ Message, Code string }
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
	if event.Type == "turn.completed" && event.Usage != nil && (event.Usage.Input > 0 || event.Usage.Output > 0) {
		events.Usage = &usage.Tokens{Input: event.Usage.Input, Output: event.Usage.Output, Cached: event.Usage.Cached}
	}
	if event.Type == "turn.failed" {
		events.ErrorStatus = codexStatus(event.Error.Message)
		events.ErrorMessage, _ = diagnostic.Text(event.Error.Message, 4096)
	}
	if event.Type == "error" {
		message := event.Message
		if event.Status != 0 || event.Error.Code != "" {
			message = string(line)
		}
		events.ErrorStatus = codexStatus(message)
		events.ErrorMessage, _ = diagnostic.Text(message, 4096)
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
