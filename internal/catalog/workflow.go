package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

var ErrNotFound = errors.New("配置对象或版本不存在")
var ErrInvalid = errors.New("配置无效")

func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }

var ErrConflict = errors.New("草稿修订已变化，请重新读取后编辑")
var ErrArchived = errors.New("配置已归档，不能修改、发布或建立新引用")

type NewObject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Spec Spec   `json:"spec"`
}

type Object struct {
	ID string `json:"id"`
	NewObject
	Revision  int       `json:"revision"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
}

type Version struct {
	ID             string    `json:"id"`
	ObjectID       string    `json:"object_id"`
	Kind           string    `json:"kind"`
	Number         int       `json:"number"`
	SourceRevision int       `json:"source_revision"`
	Spec           Spec      `json:"spec"`
	CreatedAt      time.Time `json:"created_at"`
	Archived       bool      `json:"archived"`
}

type Spec struct {
	Agent      *workflow.Node `json:"agent,omitempty"`
	Flow       *Flow          `json:"flow,omitempty"`
	Connection *Connection    `json:"connection,omitempty"`
}

type Connection struct {
	Provider  string          `json:"provider,omitempty"`
	Price     *workflow.Price `json:"price,omitempty"`
	BaseURL   string          `json:"base_url,omitempty"`
	Model     string          `json:"model"`
	SecretRef string          `json:"secret_ref,omitempty"`
}

type Flow struct {
	Editor     *FlowEditor      `json:"editor,omitempty"`
	Budget     *workflow.Budget `json:"budget,omitempty"`
	Parameters *workflow.Shape  `json:"parameters,omitempty"`
	Nodes      []Binding        `json:"nodes"`
}

type Binding struct {
	Task         string                           `json:"task,omitempty"`
	OutputSchema *workflow.Shape                  `json:"output_schema,omitempty"`
	ReadOnly     bool                             `json:"read_only,omitempty"`
	When         *workflow.Condition              `json:"when,omitempty"`
	Approval     *workflow.Approval               `json:"approval,omitempty"`
	Bindings     map[string]workflow.FieldBinding `json:"bindings,omitempty"`
	ID           string                           `json:"id"`
	AgentVersion string                           `json:"agent_version"`
	DependsOn    []string                         `json:"depends_on,omitempty"`
}

func (item NewObject) Validate() error {
	if strings.TrimSpace(item.Name) == "" || len(item.Name) > 128 {
		return errors.New("配置名称长度必须为 1 至 128 字节")
	}
	spec := item.Spec
	switch item.Kind {
	case "agent":
		if spec.Agent != nil && (len(spec.Agent.Bindings) > 0 || spec.Agent.AgentVersion != "" || spec.Agent.When != nil) {
			return errors.New("独立 Agent 不允许上游绑定或伪造发布版本；映射由工作流定义")
		}
		if spec.Agent == nil || spec.Flow != nil || spec.Connection != nil || len(spec.Agent.DependsOn) != 0 {
			return errors.New("Agent 配置只能包含独立角色配置")
		}
		return spec.Agent.Validate()
	case "workflow":
		if spec.Flow == nil || spec.Agent != nil || spec.Connection != nil {
			return errors.New("工作流配置类型错误")
		}
		return spec.Flow.validateDraft()
	case "connection":
		if spec.Connection == nil || spec.Agent != nil || spec.Flow != nil {
			return errors.New("连接配置类型错误")
		}
		return spec.Connection.Validate()
	default:
		return errors.New("未知配置类型")
	}
}

var secretRef = regexp.MustCompile(`^PANGU_SECRET_[A-Z][A-Z0-9_]{0,63}$`)

func (c Connection) Validate() error {
	if c.Provider == "codex" {
		if c.BaseURL != "" || c.SecretRef != "" || c.Price != nil || len(c.Model) > 128 || c.Model != "" && strings.TrimSpace(c.Model) == "" {
			return errors.New("Codex 账号连接仅允许可选模型名，不接受 API 地址、凭据引用或单价")
		}
		return nil
	}
	if c.Provider != "" && c.Provider != "openai" {
		return errors.New("模型连接 provider 必须为 openai 或 codex")
	}
	if err := c.Price.Validate(); err != nil {
		return err
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("模型端点必须是无凭据、查询参数和片段的 HTTP(S) 地址")
	}
	if strings.TrimSpace(c.Model) == "" || len(c.Model) > 128 || !secretRef.MatchString(c.SecretRef) {
		return errors.New("模型名或服务端 PANGU_SECRET_ 凭据引用无效")
	}
	return nil
}
