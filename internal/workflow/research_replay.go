package workflow

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"
	"unicode/utf8"
)

// ParseResearchReplay separates the model-visible brief from the frozen documents.
func ParseResearchReplay(raw string) (string, []string, error) {
	var input struct {
		Brief     json.RawMessage `json:"brief"`
		Documents []string        `json:"documents"`
	}
	if len(raw) > 16384 || !utf8.ValidString(raw) {
		return "", nil, errors.New("回放输入不可超过 16 KiB，必须是 UTF-8")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		return "", nil, errors.New("回放输入必须符合 brief 和 documents 结构")
	}
	brief, err := ParseBrief(string(input.Brief), "research")
	if err != nil {
		return "", nil, err
	}
	if len(brief.URLs)+len(brief.PublicQueries) > 0 || len(brief.Documents) == 0 || len(brief.Documents) != len(input.Documents) {
		return "", nil, errors.New("回放必须仅包含与 brief 一一对应的冻结资料")
	}
	seen := map[string]bool{}
	for i, name := range brief.Documents {
		if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:\r\n") || seen[name] || strings.TrimSpace(input.Documents[i]) == "" {
			return "", nil, errors.New("回放资料必须有唯一的相对名称和非空正文")
		}
		seen[name] = true
	}
	visible, err := json.Marshal(brief)
	return string(visible), input.Documents, err
}
