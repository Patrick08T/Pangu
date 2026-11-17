package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func reportOutput(node workflow.Node, out workflow.NodeOutput) (workflow.NodeOutput, error) {
	body := out.Text
	if len(body) > 1024*1024 || !utf8.ValidString(body) || diagnostic.Sensitive([]byte(body)) {
		return out, errors.New("完整报告超过 1 MiB、编码无效或包含敏感信息")
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(body))
	if err != nil {
		return out, err
	}
	if err = node.ReportSchema.Validate(value); err != nil {
		return out, err
	}
	out.Blockers, err = workflow.ParseEvidenceBlockers(body)
	if err != nil {
		return out, err
	}
	fields := value.(map[string]any)
	summary := map[string]any{}
	for name := range node.OutputSchema.Properties {
		if field, exists := fields[name]; exists {
			if text, ok := field.(string); ok && name == "summary" {
				field, _ = diagnostic.Text(text, 1024)
			}
			summary[name] = field
		}
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return out, err
	}
	report := workflow.Report{ID: uuid.NewString(), Title: node.ID + " 完整报告", Markdown: body}
	out.Text, out.ReportIDs = string(raw), []string{report.ID}
	out.Reports, out.ReportSHA256 = []workflow.Report{report}, workflow.ReportHash(body)
	return out, nil
}
