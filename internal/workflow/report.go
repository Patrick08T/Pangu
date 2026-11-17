package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type Revision struct {
	OriginCommit     string   `json:"origin_commit"`
	BaseCommit       string   `json:"base_commit"`
	HeadCommit       string   `json:"head_commit"`
	ChangedFiles     []string `json:"changed_files"`
	LocalPerforce    bool     `json:"local_perforce,omitempty"`
	Changelist       int64    `json:"changelist,omitempty"`
	ReviewRepository string   `json:"review_repository,omitempty"`
}

func ReportHash(body string) string {
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:])
}

func (n Node) validateReport() error {
	if n.ReportSchema == nil {
		return nil
	}
	if n.Executor != "codex" || !n.ReadOnly || n.Role != "" || n.OutputSchema == nil || n.ReportSchema.Type != "object" {
		return errors.New("报告输出需要只读 Codex 节点、对象报告结构和摘要结构")
	}
	if _, err := n.ReportSchema.Compile(); err != nil {
		return err
	}
	for name, shape := range n.OutputSchema.Properties {
		if shape.Type == "object" || shape.Type == "array" || n.ReportSchema.Properties[name].Type != shape.Type {
			return errors.New("摘要字段必须是完整报告中的同类型标量字段")
		}
	}
	return nil
}
