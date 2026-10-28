package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("配置对象或版本不存在")
var ErrInvalid = errors.New("配置无效")

func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }

var ErrConflict = errors.New("草稿修订已变化，请重新读取后编辑")
var ErrArchived = errors.New("配置已归档，不能修改、发布或建立新引用")

type NewObject struct {
	Kind      string `json:"kind"`
	ProjectID string `json:"project_id,omitempty"`
	Name      string `json:"name"`
	Spec      Spec   `json:"spec"`
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
	ProjectID      string    `json:"project_id,omitempty"`
	Number         int       `json:"number"`
	SourceRevision int       `json:"source_revision"`
	Spec           Spec      `json:"spec"`
	CreatedAt      time.Time `json:"created_at"`
	Archived       bool      `json:"archived"`
}

type Spec struct {
	Agent           *workflow.Node `json:"agent,omitempty"`
	TemplateVersion string         `json:"template_version,omitempty"`
	Flow            *Flow          `json:"flow,omitempty"`
	Connection      *Connection    `json:"connection,omitempty"`
}

type Connection struct {
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	SecretRef string `json:"secret_ref"`
}

type Flow struct {
	Parameters *workflow.Shape `json:"parameters,omitempty"`
	Nodes      []Binding       `json:"nodes"`
}

type Binding struct {
	Bindings     map[string]workflow.FieldBinding `json:"bindings,omitempty"`
	ID           string                           `json:"id"`
	AgentVersion string                           `json:"agent_version"`
	DependsOn    []string                         `json:"depends_on,omitempty"`
}

func (item NewObject) Validate() error {
	if strings.TrimSpace(item.Name) == "" || len(item.Name) > 128 {
		return errors.New("配置名称长度必须为 1 至 128 字节")
	}
	if item.Kind != "template" {
		if _, err := uuid.Parse(item.ProjectID); err != nil {
			return errors.New("配置必须绑定有效项目")
		}
	} else if item.ProjectID != "" {
		return errors.New("共享模板不能绑定项目")
	}
	spec := item.Spec
	switch item.Kind {
	case "template", "agent":
		if spec.Agent != nil && (len(spec.Agent.Bindings) > 0 || spec.Agent.AgentVersion != "") {
			return errors.New("独立 Agent 不允许上游绑定或伪造发布版本；映射由工作流定义")
		}
		if spec.Agent == nil || spec.Flow != nil || spec.Connection != nil || len(spec.Agent.DependsOn) != 0 {
			return errors.New("Agent 配置只能包含独立节点与模板版本引用")
		}
		if item.Kind == "template" && (spec.TemplateVersion != "" || spec.Agent.ConnectionVersion != "") {
			return errors.New("共享模板不能含项目连接或绑定信息")
		}
		return spec.Agent.Validate()
	case "workflow":
		if spec.Flow == nil || spec.Agent != nil || spec.Connection != nil || spec.TemplateVersion != "" {
			return errors.New("工作流配置类型错误")
		}
		if len(spec.Flow.Nodes) == 0 || len(spec.Flow.Nodes) > 32 {
			return errors.New("工作流节点数必须为 1 至 32")
		}
		for _, node := range spec.Flow.Nodes {
			if _, err := uuid.Parse(node.AgentVersion); err != nil {
				return errors.New("工作流必须引用明确的 Agent 版本 ID")
			}
		}
	case "connection":
		if spec.Connection == nil || spec.Agent != nil || spec.Flow != nil || spec.TemplateVersion != "" {
			return errors.New("连接配置类型错误")
		}
		return spec.Connection.Validate()
	default:
		return errors.New("未知配置类型")
	}
	return nil
}

var secretRef = regexp.MustCompile(`^PANGU_SECRET_[A-Z][A-Z0-9_]{0,63}$`)

func (c Connection) Validate() error {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("模型端点必须是无凭据、查询参数和片段的 HTTP(S) 地址")
	}
	if strings.TrimSpace(c.Model) == "" || len(c.Model) > 128 || !secretRef.MatchString(c.SecretRef) {
		return errors.New("模型名或服务端 PANGU_SECRET_ 凭据引用无效")
	}
	return nil
}
