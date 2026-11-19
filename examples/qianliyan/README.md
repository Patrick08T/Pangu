# 千里眼示例项目

千里眼用 Pangu 收集金融、AI 和游戏行业的公开信息，生成可追溯的中文简报。飞书和微信公众号两条分支并行准备交付记录，均引用采集节点登记的同一份报告。示例可在自己的 Worker、Git 项目、模型连接和渠道账号上使用。

## 配置与运行

1. 在 Pangu 登记一个 Worker 可访问的本地 Git 项目，选用实际分支和 Worker ID。示例也可以使用 Pangu 仓库本身作为执行项目。
2. 配置并发布共享模型连接。蓝本使用 `gpt-6.2-sol`，对应 OpenAI Responses API 和 `high` 推理强度。凭据由 Worker 环境提供。使用其他兼容连接时，把三个角色的 `model` 改为该连接记录中的模型。
3. [workflow.json](workflow.json) 是工作流蓝本。按 `agents` 中的配置创建三个 Agent，选择上一步的模型连接并发布。通过 API 导入时，以 `pangu.catalog.v1` 包装每个角色，设置 `kind` 为 `agent`、`name` 为角色名称，并将 `node` 放入 `spec.agent`。设置 `spec.agent.connection_version` 为模型连接发布版本 ID。详见 [模板与产品经理使用](../../docs/模板与产品经理使用.md)。
4. 将三个 Agent 的发布版本 ID 写入下方的 `$versions`，生成工作流导入请求。通过 `POST /api/catalog/import` 创建工作流草稿，发布后可以在画布查看编排。这样会保留两个交付角色所引用的 `collect` 节点标识。
5. 运行工作流时选择登记的项目，提交 [brief.json](brief.json) 作为输入。该输入包含固定公开网址和授权搜索词。只用固定网址时，删除 `public_queries` 即可使用不依赖 Worker Codex 搜索的路径。

```powershell
$bundle = Get-Content -Raw -Encoding UTF8 ./workflow.json | ConvertFrom-Json
$versions = @{ collect = '<采集版本ID>'; feishu = '<飞书版本ID>'; wechat = '<公众号版本ID>' }
$nodes = foreach ($step in $bundle.steps) {
    $node = @{ id = $step.id; agent_version = $versions[$step.agent] }
    if ($step.depends_on) { $node.depends_on = @($step.depends_on) }
    $node
}
@{
    format = 'pangu.catalog.v1'
    kind = 'workflow'
    name = $bundle.name
    spec = @{ flow = @{ nodes = @($nodes) } }
} | ConvertTo-Json -Depth 10
```

## 生成 HTML

在 Pangu 运行详情确认采集报告 ID。核对两条分支的 `report_id` 均为该 ID，再下载采集报告原文并使用 Node.js 24 渲染。

```powershell
node render.mjs report.md qianliyan.html
```

## 渠道交付

- 飞书分支返回 `pending_delivery`。将渲染出的 HTML 附件发送到你已授权的联系人或群组，发送状态以消息回执为准。
- 微信公众号分支返回 `pending_account`。公众号配置好接口权限和永久封面素材后，在本机安全配置 `QIANLIYAN_WECHAT_APP_ID`、`QIANLIYAN_WECHAT_APP_SECRET` 和 `QIANLIYAN_WECHAT_COVER_MEDIA_ID`。显式运行下列命令创建草稿、提交发布并查询结果。若回执不完整，先核对公众号后台，再处理重试。

```powershell
node wechat.mjs publish receipt.jsonl report.md
node wechat.mjs status receipt.jsonl
```

普通业务运行用于收集和交付。采集角色的原生回放评测可使用 `eino_research_replay_v1`，登记带费率的固定模型连接、独立保护集和保留集后进行。回放仅运行采集节点，两个渠道分支不会执行。示例项目导入后可按需单独配置生产交付与晋级策略。
