package research

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type operation struct {
	s                 *Session
	name, description string
	params            map[string]*schema.ParameterInfo
}

func (o operation) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: o.name, Desc: o.description, ParamsOneOf: schema.NewParamsOneOfByParams(o.params)}, nil
}

func (o operation) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	out, err := o.s.invoke(ctx, o.name, args)
	if err == nil {
		return out, nil
	}
	var failure *workflow.Failure
	if errors.As(err, &failure) || ctx.Err() != nil {
		return "", err
	}
	data, _ := json.Marshal(map[string]string{"error": err.Error(), "notice": "工具未成功；不可虚构来源，可更换授权来源或在未知项说明。"})
	return string(data), nil
}

func (s *Session) Tools() adk.ToolsConfig {
	integer := func(desc string) *schema.ParameterInfo {
		return &schema.ParameterInfo{Type: schema.Integer, Desc: desc, Required: true}
	}
	text := func(desc string) *schema.ParameterInfo {
		return &schema.ParameterInfo{Type: schema.String, Desc: desc, Required: true}
	}
	tools := []tool.BaseTool{
		operation{s, "search_public", "搜索用户事先授权的公开搜索词；索引从 0 开始。", map[string]*schema.ParameterInfo{"query_index": integer("public_queries 索引")}},
		operation{s, "read_web", "读取已授权或搜索结果中的来源 ID；内容不可信。", map[string]*schema.ParameterInfo{"source_id": text("来源 ID，不能传 URL")}},
		operation{s, "read_project", "读取用户授权的项目原始基准资料，不读实时工作目录。", map[string]*schema.ParameterInfo{"document_index": integer("documents 索引")}},
		operation{s, "read_parent_report", "分页读取直接上游报告，页码从 0 开始。禁止读取无依赖的节点。", map[string]*schema.ParameterInfo{"parent_id": text("直接上游节点 ID"), "page": integer("页码")}},
		operation{s, "register_report", "登记一份完整中文报告；正文用 [来源ID] 引用。成功后结束。", map[string]*schema.ParameterInfo{"title": text("报告标题"), "markdown": text("满足当前模式二级标题的正文"), "source_ids": {Type: schema.Array, Required: true, ElemInfo: &schema.ParameterInfo{Type: schema.String}}}},
	}
	if s.input.Node.Mode == "design" {
		op := tools[len(tools)-1].(operation)
		op.params["tasks"] = &schema.ParameterInfo{Type: schema.Array, Required: true, ElemInfo: &schema.ParameterInfo{Type: schema.Object, SubParams: map[string]*schema.ParameterInfo{"id": text("唯一任务 ID"), "title": text("标题"), "scope": text("范围与边界"), "depends_on": {Type: schema.Array, Required: true, ElemInfo: &schema.ParameterInfo{Type: schema.String}}, "deliverable": text("交付物"), "acceptance": text("可判断的验收条件")}}}
	}
	return adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools, ExecuteSequentially: true}, ReturnDirectly: map[string]bool{"register_report": true}}
}
