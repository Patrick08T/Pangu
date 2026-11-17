# Perforce 本地工作区：验证及 OCR 审查

日期：2026-10-30。用户确认直接在绑定的本地 P4 工作区执行；保留原有未提交修改和清单归属，执行前留快照，新打开文件归入本轮清单。

## 实施与验证

- 已新增登记与编辑绑定目录的页面、管理员接口、Worker 预检、本地执行、外部现场快照、失败恢复和精确清单交付。
- TDD 红灯已复现：缺目录仍可登记、只读违规修改未跟踪、中断后未补登记、已打开 edit 文件删除后动作未更新、无关文件混入本轮 shelf、AltRoot 读取主 Root。对应测试修复后通过。
- 独立 Unicode P4D 集成测试通过，覆盖 classic/stream、中文与二进制文件、真实本地目录读写、已有修改、原清单归属、失败与进程中断、只读违规、AltRoot、交付幂等与范围、绑定冲突及待派发阻止。测试服务已停止。
- 全量 Go 测试通过（独立测试数据库，包串行）；go vet 通过；架构与单函数 70 行限制通过。旧 OpenAPI 测试按新目录必填契约更新后通过。
- 前端 46 个文件、139 项测试通过，类型检查、契约同步、生产构建通过。
- 平台证据临时目录测试通过：本地工作区不创建 .git，索引可读取，篡改会拒绝，结束清理临时目录。

## 部署核验

- 更新前分页检查 22 条运行，无活跃任务。旧二进制已备份。首次启动检测待迁移后退出；显式应用 0053、0054、0055 后正常启动。
- 新服务 PID 105424，API 与 Worker 均启动；认证后的 /api/readyz 返回 200，database 与 hatchet_registration 为 ok。
- Partopia Client taotaopeng_partopia_main 的 Root 与 Host 已只读核验。项目目录通过新接口保存为 E:\Unity\taotaopeng_partopia_main，返回 200。
- 新二进制 SHA256：BB11CE787CB229EA57E88CDD72412BF18598B81C88206A35F9A12E35F35C982E。默认模型仍为 GPT-6-Astra / xhigh。
- 未运行真实 Partopia 代码修改工作流；真实项目仅核验并保存绑定配置。没有提交或推送 Git。

## OCR delegate 审查

使用 C:/Users/pengtaotao/.codex/skills/open-code-review-delegate/SKILL.md；执行 ocr delegate preview --format json 与 ocr delegate rule --format json，当前代理逐文件审查，不调用外部模型端点。

preview 总计 320 项：reviewable 165 项、工具排除 155 项；total_files=165，reviewed_files=33，skipped_files=132，coverage_rate=20.00%。
本轮实际变更 44 个代码、迁移、契约及前端文件；33 项位于 OCR 默认范围，另有 11 个测试文件补充审查。本轮增量覆盖 100%。历史未改动的 132 项逐一标记 skipped，未冒称审查全部历史改动。

检查重点：目录与权限、原始配置读取、快照与中断恢复、锁和执行器停止确认、清单归属、交付范围、历史保留、前后端契约及负面测试。审查发现的 AltRoot 目录选择、只读违规追踪、原清单删除动作、无关文件混入 shelf、待派发任务改绑及目录可见性问题已修复并验证。当前本轮范围没有未解决的高或中风险代码发现。

仍需注意：平台无法禁止外部编辑器并发改同一目录；运行期间的外部改动可能进入快照。P4 元数据大小、差异文件数和既有命令超时限制仍生效，大项目建议缩小 scope。

### 本轮逐文件覆盖

| 文件 | 状态 |
| --- | --- |
| internal/agent/codex.go | reviewed（本轮增量与上下文） |
| internal/agent/model_options_test.go | reviewed（本轮增量与上下文） |
| internal/api/identity_test.go | reviewed（本轮增量与上下文） |
| internal/api/perforce.go | reviewed（本轮增量与上下文） |
| internal/api/perforce_test.go | reviewed（本轮增量与上下文） |
| internal/database/migrations/0053_perforce_local.sql | reviewed（本轮增量与上下文） |
| internal/database/migrations/0054_perforce_local_delivery.sql | reviewed（本轮增量与上下文） |
| internal/database/migrations/0055_perforce_local_tracking.sql | reviewed（本轮增量与上下文） |
| internal/perforce/client.go | reviewed（本轮增量与上下文） |
| internal/perforce/delivery.go | reviewed（本轮增量与上下文） |
| internal/perforce/local.go | reviewed（本轮增量与上下文） |
| internal/perforce/local_delivery.go | reviewed（本轮增量与上下文） |
| internal/perforce/workspace.go | reviewed（本轮增量与上下文） |
| internal/scheduler/execution.go | reviewed（本轮增量与上下文） |
| internal/scheduler/local_evidence.go | reviewed（本轮增量与上下文） |
| internal/scheduler/local_evidence_test.go | reviewed（本轮增量与上下文） |
| internal/scheduler/perforce.go | reviewed（本轮增量与上下文） |
| internal/scheduler/workspace.go | reviewed（本轮增量与上下文） |
| internal/workflow/report.go | reviewed（本轮增量与上下文） |
| internal/workspace/artifact.go | reviewed（本轮增量与上下文） |
| internal/workspace/database.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_artifact.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_binding.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_delivery.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_delivery_test.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_execute.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_integration_test.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_local_test.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_queue.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_recovery.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_review.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_revision.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_snapshot.go | reviewed（本轮增量与上下文） |
| internal/workspace/perforce_test.go | reviewed（本轮增量与上下文） |
| internal/workspace/project.go | reviewed（本轮增量与上下文） |
| internal/workspace/revision.go | reviewed（本轮增量与上下文） |
| api/openapi.yaml | reviewed（本轮增量与上下文） |
| api/perforce_test.go | reviewed（本轮增量与上下文） |
| web/src/api/schema.d.ts | reviewed（本轮增量与上下文） |
| web/src/identity/project.test.tsx | reviewed（本轮增量与上下文） |
| web/src/identity/project.tsx | reviewed（本轮增量与上下文） |
| web/src/runs/perforce.test.tsx | reviewed（本轮增量与上下文） |
| web/src/runs/perforce.tsx | reviewed（本轮增量与上下文） |

### 历史增量跳过清单

| 文件与 Git 状态 | 审查状态及原因 |
| --- | --- |
| examples/workflows/game-backend-performance.json (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| examples/workflows/market-driven-iteration.json (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/codex_model.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/eino.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/models.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/policy.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/preflight.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/usage.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/workflow.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/approval.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/audit.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/catalog.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/catalog_query.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/delivery.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/evaluation_case.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/factory.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/hatchet.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/list.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/webhook.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/database.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/event.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/identity.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/database.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/editor.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/reference.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/version.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/workflow.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/delivery/catalog.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/execution/cancel.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/factory/workflow.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/research/database.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/research/parent.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/approval.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/branch.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/cancel.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/contract.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/fork.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/hatchet.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/history.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/rerun.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/run.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/submission/database.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/submission/dispatch.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/trace/agent.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/usage/database.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/usage/http.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/usage/query.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/approval.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/branch.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/definition.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/fork.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/hatchet.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/models.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/policy.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/schema.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workspace/fork.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workspace/retention.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workspace/unprepared.go (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/App.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/agent/codex.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/agent/configuration.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/agent/model.ts (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/workflow.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/catalog/list.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/console/components.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/console/routes.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/factory/preview.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/assistant.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/canvas.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/catalog.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/editor.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/model.ts (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/panel.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/flow/style.css (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/debug.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/detail.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/execution.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/fork.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/list.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/workflow.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/style.css (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/usage/workflow.tsx (modified) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/model_options.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/agent/report.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/evidence.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/project_query.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/api/repair.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/continuation.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/plan.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/query.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/repair.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/repair_continuation.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/report.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/review.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/round.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/approval/successor.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/plan.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/catalog/repair.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0045_approval_wait.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0046_approval_review.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0047_plan_continuation.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0048_evidence_repair.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0049_evidence_repair_retention.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0050_evidence_repair_revisions.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0051_catalog_updated.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/database/migrations/0052_perforce.sql (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/delivery/approval.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/evidence/database.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/evidence/record.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/evidence/workspace.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/execution/retry.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/blocker.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/continuation.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/repair.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/scheduler/retry.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/usage/continuation.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/blocker.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/evidence.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/outcome.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/plan.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/review.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/review_choice.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workflow/review_schema.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| internal/workspace/report.go (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| scripts/test-perforce.ps1 (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/choice.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/configuration.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/plan.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/repair.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/approval/review.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/evidence.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |
| web/src/runs/outcome.tsx (added) | skipped：本轮开始前已存在且本轮未修改；不重复审查历史任务增量 |

### 证据文件

- .tools/perforce-local-ocr-preview.json：完整文件清单及工具排除原因。
- .tools/perforce-local-ocr-rules.json：每个文件对应的 OCR 规则组。
- .tools/perforce-local-ocr-checklist.json：每个 (path,status) 的 reviewed/skipped 与理由。
- .tools/perforce-local-before：本轮修改前基线，用于分离此前尚未提交的任务。
- .tools/perforce-local-go-tests.txt：最终全量 Go 测试结果。
- .tools/p4/test-8218edc24b35494d8dd7ebbc11548ac8：最后一轮通过的独立 P4D 测试服务日志。

上线页面核验：项目切换为 Partopia 后显示已保存的根目录和编辑表单；新项目选择 Perforce 后，本地工作区根目录为必填。页面状态见 .tools/perforce-local-ui.txt。
