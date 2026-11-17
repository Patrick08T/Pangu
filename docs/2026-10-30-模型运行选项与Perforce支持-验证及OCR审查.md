# 模型运行选项与 Perforce 支持：验证及 OCR 审查

日期：2026-10-30。依据用户已批准的设计实施，沿用 TDD、Git-only、不提交或推送。

## 使用入口

- Agent 角色 → 选择角色 → 模型与执行：原生 Codex 选择具体模型后，可以设置推理强度和 Fast。强度候选来自当前 Worker 的实际模型目录；Fast 区分运行环境默认、开启、关闭。
- Eino 的模型由已发布的共享连接版本固定。在设置 → 模型连接中指定具体 Codex 模型并发布，再回到角色选择该连接版本，设置强度和 Fast。页面已补充入口及提示。普通模型 API 不接受 Codex 专属选项。
- 保存并发布新的 Agent 版本后，工作流需要选择该版本。历史运行快照不被改写。
- 设置 → 项目与成员 → 登记项目：来源类型选择 Perforce，填写 Worker ID、服务器、用户、模板 Client 和项目子目录；可先检查连接。凭据使用 Worker 的 P4 登录票据，表单不保存密码。
- Perforce 运行结束后，在运行详情的 Perforce 交付区生成待审核 changelist。操作异步执行，页面查询进度；完成后显示编号，供 P4V 查看 shelved files。不自动 submit。

## 实现与验证

模型选项贯通页面、API、固定版本、运行节点及实际 Codex 参数，显式关闭 Fast 不会被默认优先档位覆盖。预检依据实际模型能力拒绝不支持的组合。没有新增模型目录缓存。

Perforce 首次运行固定已提交基线、模板映射和项目范围；创建平台自有 Client，在私有 Git 检查点中执行节点。续跑与修订继承原来源和成果。交付相对于最初 P4 基线导出精确差异，复用交付记录与 changelist。待审核 shelf 及关联 Client 不参与普通过期清理。

| 验证 | 结果 |
| --- | --- |
| 全量 Go 测试：独立测试库，`go test -p 1 ./...` | 通过 |
| Go 静态检查：`go vet ./...` | 通过 |
| 最后并发边界修复后的 workspace、api、architecture 回归 | 通过；新增测试先复现空成功回执，再验证拒绝繁忙工作区 |
| 前端 Vitest | 46 个文件、137 项测试通过 |
| 前端契约生成一致性、TypeScript 检查、生产构建 | 通过；最后提示文案修改后重新构建 |
| Go 最终可执行程序构建 | 通过，包含最新嵌入前端 |
| Git 差异空白检查 | 通过 |
| 临时非 Unicode 和 Unicode P4D 实测 | 通过，classic 和 stream 均覆盖 |
| P4 基线隔离、日常未提交改动隔离、续跑、修订复用成果 | 通过 |
| P4 精确增加／修改／删除、空格及中文文件名、二进制 shelf 内容 | 通过 |
| P4 交付回执丢失重试、重复请求、待审核交付保留 | 通过 |
| 实际 API → Hatchet → 绑定 Worker 的只读 P4 连接检查 | 200，ready=true；目标为本机临时测试服务器 |
| 实际浏览器 | 原生模型的 high 与 Fast 关闭可选择；三态完整；Perforce 登记字段正常；临时选择未保存 |
| 原有项目迁移 | Pangu 和 LiWork 均正确显示 Git |

真实 P4 测试入口：`scripts/test-perforce.ps1`，支持 `-Unicode`。自动创建回环地址上的临时服务器、独立测试 Client 与 depot；日志保留，服务器在收尾时停止。最后一次 Unicode 集成日志位于 `.tools/p4/test-ea62e82aca2249da91478ab5788634e7`。额外手动诊断实例也已停止。

本次没有在公司 depot 中创建测试改动，没有修改日常 Client，也没有登记尚未明确范围的正式 Perforce 项目。未额外发起真实模型付费调用；执行参数由 CLI 测试替身、桥接回归及实际模型目录验证。完整交付在真实临时 P4 服务和独立测试数据库上验证；线上任务路由通过只读连接检查验证。

## 审查方法与发现

遵循 `C:/Users/pengtaotao/.codex/skills/open-code-review-delegate/SKILL.md`，执行 `ocr delegate preview --format json` 和 `ocr delegate rule --format json <逐个文件参数>`，由当前代理审查；未调用外部模型端点。原始结果保存在 `.tools/model-perforce-ocr-preview.json` 和 `.tools/model-perforce-ocr-rules.json`。

审查聚焦本轮模型选项和 Perforce 的生产代码、API 契约、迁移、前端与测试脚本。工作区包含此前任务遗留的大量改动，下面逐项记录实际范围，未将那些历史改动计为已审查。

| path | content | category | severity | 处理与验证 |
| --- | --- | --- | --- | --- |
| internal/scheduler/perforce.go | 同步交付可能超过 HTTP 响应时限，页面报错而后台仍在操作。 | bug | high | 改为持久任务，API 返回 202，页面轮询；API 回归通过。 |
| internal/perforce/workspace.go | 同步目标链接或重试残留可能污染来源。 | security | high | 同步前核验真实路径和平台归属；来源固定基线强制同步并检查额外文件和打开记录。 |
| internal/workspace/retention.go | 清理自有 Client 必须在运行过期和活跃状态门槛之后执行。 | bug | high | 顺序调整，待审核交付阻止清理；保留测试通过。 |
| internal/perforce/client.go | Windows 的命令行编码与 Unicode P4 文件参数不一致，可能破坏中文路径。 | bug | high | Unicode 文件参数通过 UTF-8 标准输入传递；中文二进制 shelf 实测通过。 |
| internal/workspace/perforce_delivery.go | 修订续跑交付必须包含复用成果，不能只比较本次分支基准。 | bug | high | 与最初 P4 导入提交比较，修订交付测试通过。 |
| internal/workspace/perforce_queue.go | 工作区繁忙且尚无交付记录时可能返回空的成功回执。 | bug | medium | 返回 ErrLeased；新增测试红灯复现后绿灯通过。 |
| web/src/agent/codex.tsx | 默认模型的 Eino 连接禁用选项时，缺少前往具体模型配置的指引。 | other | low | 增加共享连接的配置、发布和重新选择说明与入口。 |

以上发现均已处理；本轮审查范围内未遗留已知阻断问题。

## 运行边界

- 沿用产物边界：最多 1000 个差异文件、单文件最大 10 MiB；不支持符号链接、特殊文件或现有文件可执行权限变化。
- 非 Unicode P4 服务仅接受可靠的 ASCII 路径和表单；中文文件路径需要 Unicode 服务。用户与 Client 标识采用安全字符校验。
- Stream 映射变化时会拒绝交付，避免把成果写入与基线不同的映射。
- 已创建的待审核交付需要保留，不能依赖普通运行过期自动释放其 Client。
- Worker 不可用或任务中断时可重试交付；已形成的 shelf／编号会复用。没有承诺无人值守自动 submit。

## 本地服务更新

更新前确认 22 条历史运行全部结束。旧程序及进程记录保存在 `.tools/backups/model-perforce-20261030/`。生产数据库已应用 0052 迁移，测试使用独立数据库。

当前服务 PID：116156；端口：8081；模式：all。

最终程序 SHA-256：`57D989A8406B5160F1C41BFD5AD16DE530599A8E58B29468BFB7722DA79247ED`。

健康和就绪接口已通过，Hatchet 注册正常。回退不能仅覆盖旧程序：旧版没有必填 vcs 字段，若回退还需处理数据库契约及新 Perforce 项目的使用范围。当前以向前修复为优先。

## OCR 文件覆盖清单

以下以 path 和 status 共同标识文件，覆盖率以 OCR 可审查文件为分母。

- total_files：303；reviewable_files：152；reviewed_files：36；skipped_files：116；excluded_files：151。
- coverage_rate：23.68%；本轮指定范围覆盖 36/36；preview 清单归档率 100%。

| 文件 | Git 状态 | 审查结果 | 说明 |
| --- | --- | --- | --- |
| api/openapi.yaml | modified | reviewed | 已审查本轮相关改动及上下文 |
| examples/workflows/game-backend-performance.json | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| examples/workflows/market-driven-iteration.json | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/agent/codex.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/codex_model.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/eino.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/models.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/policy.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/agent/preflight.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/usage.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/agent/workflow.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/approval.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/audit.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/catalog.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/catalog_query.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/delivery.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/evaluation_case.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/factory.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/hatchet.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/api/list.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/api/webhook.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/database.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/event.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/identity.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/database.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/editor.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/reference.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/catalog/version.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/workflow.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/delivery/catalog.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/execution/cancel.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/factory/workflow.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/research/database.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/research/parent.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/approval.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/branch.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/cancel.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/contract.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/execution.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/fork.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/hatchet.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/scheduler/history.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/rerun.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/run.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/workspace.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/submission/database.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/submission/dispatch.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/trace/agent.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/usage/database.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/usage/http.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/usage/query.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/approval.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/branch.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/definition.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workflow/fork.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/hatchet.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/models.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workflow/policy.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/schema.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workspace/artifact.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/database.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/fork.go | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workspace/project.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/retention.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/unprepared.go | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/App.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/agent/codex.tsx | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/agent/configuration.tsx | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/agent/model.ts | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/api/schema.d.ts | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/approval/workflow.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/catalog/list.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/console/components.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/console/routes.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/factory/preview.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/assistant.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/canvas.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/catalog.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/editor.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/model.ts | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/panel.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/flow/style.css | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/identity/project.tsx | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/runs/debug.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/detail.tsx | modified | reviewed | 已审查本轮相关改动及上下文 |
| web/src/runs/execution.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/fork.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/list.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/workflow.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/style.css | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/usage/workflow.tsx | modified | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/agent/model_options.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/agent/report.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/evidence.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/perforce.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/api/project_query.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/api/repair.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/continuation.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/plan.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/query.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/repair.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/repair_continuation.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/report.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/review.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/round.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/approval/successor.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/plan.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/catalog/repair.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0045_approval_wait.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0046_approval_review.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0047_plan_continuation.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0048_evidence_repair.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0049_evidence_repair_retention.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0050_evidence_repair_revisions.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0051_catalog_updated.sql | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/database/migrations/0052_perforce.sql | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/delivery/approval.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/evidence/database.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/evidence/record.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/evidence/workspace.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/execution/retry.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/perforce/client.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/perforce/delivery.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/perforce/workspace.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/scheduler/blocker.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/continuation.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/perforce.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/scheduler/repair.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/scheduler/retry.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/usage/continuation.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/blocker.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/evidence.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/outcome.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/plan.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/report.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/review.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/review_choice.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workflow/review_schema.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workspace/perforce.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/perforce_delivery.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/perforce_queue.go | added | reviewed | 已审查本轮相关改动及上下文 |
| internal/workspace/report.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| internal/workspace/revision.go | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| scripts/test-perforce.ps1 | added | reviewed | 已审查本轮相关改动及上下文 |
| web/src/approval/choice.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/approval/configuration.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/approval/plan.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/approval/repair.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/approval/review.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/evidence.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/outcome.tsx | added | skipped | 此前任务遗留改动，不在本轮模型运行选项与 Perforce 实施范围；本轮未审查 |
| web/src/runs/perforce.tsx | added | reviewed | 已审查本轮相关改动及上下文 |

### OCR 默认排除项

| 文件 | 原因 |
| --- | --- |
| docs/市场驱动产品迭代工作流-2026-10-28.md | OCR 不支持的文件扩展名（含文档） |
| internal/agent/codex_model_test.go | 默认路径过滤（含测试文件） |
| internal/agent/codex_test.go | 默认路径过滤（含测试文件） |
| internal/agent/event_test.go | 默认路径过滤（含测试文件） |
| internal/agent/policy_test.go | 默认路径过滤（含测试文件） |
| internal/api/approval_test.go | 默认路径过滤（含测试文件） |
| internal/api/authorization_test.go | 默认路径过滤（含测试文件） |
| internal/api/catalog_preflight_test.go | 默认路径过滤（含测试文件） |
| internal/api/catalog_query_test.go | 默认路径过滤（含测试文件） |
| internal/api/catalog_submission_test.go | 默认路径过滤（含测试文件） |
| internal/api/catalog_test.go | 默认路径过滤（含测试文件） |
| internal/api/catalog_trial_test.go | 默认路径过滤（含测试文件） |
| internal/api/delivery_test.go | 默认路径过滤（含测试文件） |
| internal/api/factory_test.go | 默认路径过滤（含测试文件） |
| internal/api/identity_test.go | 默认路径过滤（含测试文件） |
| internal/api/webhook_test.go | 默认路径过滤（含测试文件） |
| internal/api/workspace_test.go | 默认路径过滤（含测试文件） |
| internal/approval/database_test.go | 默认路径过滤（含测试文件） |
| internal/approval/diagnostic_test.go | 默认路径过滤（含测试文件） |
| internal/approval/expiry_test.go | 默认路径过滤（含测试文件） |
| internal/architecture/architecture_test.go | 默认路径过滤（含测试文件） |
| internal/artifact/export_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/bundle_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/database_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/market_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/performance_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/shared_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/template_test.go | 默认路径过滤（含测试文件） |
| internal/delivery/workflow_test.go | 默认路径过滤（含测试文件） |
| internal/evaluation/database_test.go | 默认路径过滤（含测试文件） |
| internal/factory/database_test.go | 默认路径过滤（含测试文件） |
| internal/factory/optimization_test.go | 默认路径过滤（含测试文件） |
| internal/identity/authorization_test.go | 默认路径过滤（含测试文件） |
| internal/identity/schedule_test.go | 默认路径过滤（含测试文件） |
| internal/identity/token_race_test.go | 默认路径过滤（含测试文件） |
| internal/schedule/database_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/approval_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/backup_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/contract_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/delivery_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/factory_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/fork_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/hatchet_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/verification_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/workspace_integration_test.go | 默认路径过滤（含测试文件） |
| internal/submission/database_test.go | 默认路径过滤（含测试文件） |
| internal/trace/agent_test.go | 默认路径过滤（含测试文件） |
| internal/usage/database_test.go | 默认路径过滤（含测试文件） |
| internal/usage/http_test.go | 默认路径过滤（含测试文件） |
| internal/usage/unlimited_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/approval_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/fork_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/schema_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/database_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/fork_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/lifecycle_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/recovery_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/retention_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/unprepared_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/verification_test.go | 默认路径过滤（含测试文件） |
| specs/运行契约.md | OCR 不支持的文件扩展名（含文档） |
| web/src/agent/editor.test.tsx | 默认路径过滤（含测试文件） |
| web/src/approval/workflow.test.tsx | 默认路径过滤（含测试文件） |
| web/src/catalog/editor.test.tsx | 默认路径过滤（含测试文件） |
| web/src/console/session.test.tsx | 默认路径过滤（含测试文件） |
| web/src/flow/assistant.test.tsx | 默认路径过滤（含测试文件） |
| web/src/flow/canvas.test.tsx | 默认路径过滤（含测试文件） |
| web/src/flow/editor.test.tsx | 默认路径过滤（含测试文件） |
| web/src/identity/project.test.tsx | 默认路径过滤（含测试文件） |
| web/src/runs/detail.test.tsx | 默认路径过滤（含测试文件） |
| api/perforce_test.go | 默认路径过滤（含测试文件） |
| docs/2026-10-29-迭代闭环改进-OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-29-迭代闭环改进-验证与上线.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-工作流更新时间与清理-验证及OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-工厂历史对话修复-验证及OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-市场迭代自动化范围-验证与OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-模型运行选项与Perforce支持-验证及OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-编排节点角色展示-验证及OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/2026-10-30-补证修正续跑-验证与OCR审查.md | OCR 不支持的文件扩展名（含文档） |
| docs/superpowers/plans/2026-10-30-模型运行选项与Perforce项目支持.md | OCR 不支持的文件扩展名（含文档） |
| docs/superpowers/specs/2026-10-29-结构化人工审核设计.md | OCR 不支持的文件扩展名（含文档） |
| docs/superpowers/specs/2026-10-29-迭代闭环改进-design.md | OCR 不支持的文件扩展名（含文档） |
| docs/superpowers/specs/2026-10-30-模型运行选项与Perforce项目支持-design.md | OCR 不支持的文件扩展名（含文档） |
| docs/superpowers/specs/2026-10-30-补证修正续跑-design.md | OCR 不支持的文件扩展名（含文档） |
| docs/人工审批持久等待-2026-10-29.md | OCR 不支持的文件扩展名（含文档） |
| docs/多方案人工审核-2026-10-29.md | OCR 不支持的文件扩展名（含文档） |
| docs/旧版本重跑校验修复-2026-10-29.md | OCR 不支持的文件扩展名（含文档） |
| docs/模型临时故障自动重试-2026-10-29.md | OCR 不支持的文件扩展名（含文档） |
| docs/结构化人工审核使用与验证-2026-10-29.md | OCR 不支持的文件扩展名（含文档） |
| internal/agent/model_options_test.go | 默认路径过滤（含测试文件） |
| internal/agent/report_test.go | 默认路径过滤（含测试文件） |
| internal/agent/retry_test.go | 默认路径过滤（含测试文件） |
| internal/api/evidence_test.go | 默认路径过滤（含测试文件） |
| internal/api/project_query_test.go | 默认路径过滤（含测试文件） |
| internal/api/repair_test.go | 默认路径过滤（含测试文件） |
| internal/approval/choice_test.go | 默认路径过滤（含测试文件） |
| internal/approval/continuation_test.go | 默认路径过滤（含测试文件） |
| internal/approval/identity_test.go | 默认路径过滤（含测试文件） |
| internal/approval/plan_test.go | 默认路径过滤（含测试文件） |
| internal/approval/repair_test.go | 默认路径过滤（含测试文件） |
| internal/approval/report_test.go | 默认路径过滤（含测试文件） |
| internal/approval/review_test.go | 默认路径过滤（含测试文件） |
| internal/approval/wait_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/plan_test.go | 默认路径过滤（含测试文件） |
| internal/catalog/updated_test.go | 默认路径过滤（含测试文件） |
| internal/database/approval_test.go | 默认路径过滤（含测试文件） |
| internal/evidence/database_test.go | 默认路径过滤（含测试文件） |
| internal/evidence/redaction_test.go | 默认路径过滤（含测试文件） |
| internal/evidence/workspace_test.go | 默认路径过滤（含测试文件） |
| internal/perforce/workspace_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/blocker_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/choice_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/continuation_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/model_retry_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/outcome_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/repair_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/report_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/rerun_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/retry_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/retry_review_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/review_integration_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/review_test.go | 默认路径过滤（含测试文件） |
| internal/scheduler/revision_test.go | 默认路径过滤（含测试文件） |
| internal/usage/retry_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/blocker_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/continuation_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/model_options_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/outcome_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/plan_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/report_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/retry_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/review_choice_test.go | 默认路径过滤（含测试文件） |
| internal/workflow/review_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/perforce_delivery_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/perforce_integration_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/perforce_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/research_fork_test.go | 默认路径过滤（含测试文件） |
| internal/workspace/t102396t37800t52.tmp | OCR 不支持的文件扩展名（含文档） |
| internal/workspace/t104932t96696t90.tmp | OCR 不支持的文件扩展名（含文档） |
| internal/workspace/t107620t116024t12.tmp | OCR 不支持的文件扩展名（含文档） |
| internal/workspace/t61928t106516t94.tmp | OCR 不支持的文件扩展名（含文档） |
| internal/workspace/t82660t114156t42.tmp | OCR 不支持的文件扩展名（含文档） |
| web/src/agent/codex.test.tsx | 默认路径过滤（含测试文件） |
| web/src/approval/choice.test.tsx | 默认路径过滤（含测试文件） |
| web/src/approval/plan.test.tsx | 默认路径过滤（含测试文件） |
| web/src/approval/repair.test.tsx | 默认路径过滤（含测试文件） |
| web/src/approval/review.test.tsx | 默认路径过滤（含测试文件） |
| web/src/catalog/list.test.tsx | 默认路径过滤（含测试文件） |
| web/src/flow/catalog.test.tsx | 默认路径过滤（含测试文件） |
| web/src/runs/outcome.test.tsx | 默认路径过滤（含测试文件） |
| web/src/runs/perforce.test.tsx | 默认路径过滤（含测试文件） |
