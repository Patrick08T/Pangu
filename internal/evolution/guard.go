package evolution

import (
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

var (
	ErrInvalid   = errors.New("进化配置或输入无效")
	ErrForbidden = errors.New("进化操作未经授权")
	ErrConflict  = errors.New("进化代际、基线或修订冲突")
	ErrNotFound  = errors.New("进化记录不存在")
	ErrIsolation = errors.New("没有获准的受控无工具执行环境")
	ErrChecker   = errors.New("缺少受信确定性检查器")
	ErrBudget    = errors.New("独立学习或评测预算不足")
)

type Environment struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	Revision          int64  `json:"revision"`
	Backend           string `json:"backend"`
	ConnectionVersion string `json:"connection_version"`
	Enabled           bool   `json:"enabled"`
	Currency          string `json:"currency"`
	SourceBaseline    string `json:"source_baseline"`
	ConfigHash        string `json:"config_hash"`
	Hash              string `json:"-"`
}

// 只能由固定无工具backend执行，不把目录、临时HOME或调用方布尔值当隔离证明。
func validateExecution(env Environment, d workflow.Definition) error {
	if !env.Enabled || env.Backend != "eino_json_v1" || env.ConnectionVersion == "" || len(d.Nodes) == 0 {
		return ErrIsolation
	}
	for _, n := range d.Nodes {
		if n.Executor != "eino" || n.ConnectionVersion != env.ConnectionVersion || n.Approval != nil || n.Policy != nil {
			return ErrIsolation
		}
	}
	if _, err := d.Layers(); err != nil {
		return ErrIsolation
	}
	return nil
}
