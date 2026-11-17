# 模型运行选项与 Perforce 项目支持设计

> 后续用户已确认改为绑定现有本地 P4 工作区直接执行。本文的独立 Client 章节仅记录原设计，新运行以 [Perforce 本地工作区执行设计](2026-10-30-Perforce本地工作区执行-design.md) 为准；模型配置设计不变。

## 已确认的需求

- “模型子类型”指推理强度。
- 模型设置需要支持 Fast 开关。
- 项目来源新增 Perforce。
- Perforce 使用独立工作区，输出待审核 changelist，保持日常工作区中的文件和未提交改动不受影响。
- 沿用已确认的 TDD、中文文档、Git-only 开发和 OCR delegate 审查；本次不提交或推送 Pangu 仓库。

## 当前实现与改动边界

模型列表来自 Worker 当前 Codex 程序的原始内置目录。目录已有 `supported_reasoning_levels`、`default_reasoning_level` 和 `service_tiers`；Pangu 当前只提取名称与 ID，执行也只传递 `--model`。模型运行选项必须同时贯通页面、API、固定 Agent 版本、运行快照及执行命令，不能仅添加界面控件。

项目当前只支持本地 Git 分支。运行工作区、节点检查点、差异产物和续跑都基于平台私有 Git 仓库。Perforce 在代码来源与结果交付处接入，运行内部继续复用现有检查点机制，避免重新实现另一套失败恢复流程。

## 模型配置

1. 在“模型与执行”增加“推理强度”和“Fast 模式”。强度选项来自所选模型实际支持的列表，并标注目录中的默认项；不把所有模型硬编码为同一组档位。
2. 未设置强度时沿用运行环境默认。Fast 提供“运行环境默认、开启、关闭”，保证关闭能覆盖 Worker 的默认开启状态。
3. 选择具体模型后才能设置自定义强度或 Fast；切换模型后对不支持的组合给出明确校验提示，不静默替换用户选择。
4. 原生 Codex 和 Eino 使用 Codex 账号模型的路径均支持。普通模型 API 不伪装支持 Codex 专属 Fast，暂不扩大其服务商能力范围。
5. 运行时将强度传入 `model_reasoning_effort`；Fast 开启传入优先服务档位，关闭传入普通档位。参数不拼接 shell 命令。
6. 从实际执行程序读取能力并校验；不持久缓存模型目录。修改选项后仍保存、发布新 Agent 版本，由工作流明确选择该版本；历史快照保持原样。

## Perforce 项目登记

项目页增加“来源类型：Git／Perforce”。Git 保留本地路径与分支配置；Perforce 填写服务器地址、用户、模板 Client、要运行的项目子目录和 Worker ID。子目录限制运行范围，避免默认同步大型仓库的全部资产。

凭据由 Worker 现有 P4 登录票据提供，页面和数据库不保存密码。登记前可执行只读连接检查，验证登录状态、模板 Client、映射和项目范围。Git 与 P4 各自校验必需字段，不接受混合配置。

已存 Git 项目通过数据库迁移明确标记为 Git。新增来源字段贯通项目 API、界面、预检与运行详情；不新增隐式兼容分支。

## Perforce 运行与续跑

1. 每次首次运行解析所选范围的最新已提交 changelist，并保存基线号、服务器、用户、模板映射和范围快照。首次不导入日常工作区未提交的文件。
2. 依据模板映射创建平台拥有的独立 Client，Root 必须位于 Worker 托管运行目录，只同步已登记范围到固定基线。普通 Client 与 Stream Client 的映射都以 P4 元数据为准；无法安全解析的映射明确拒绝，不能扩大范围。
3. 将该基线导入平台内部 Git 检查点仓库。各 Agent 在独立节点副本中执行，现有检查点、审批、重试、差异证据和验收继续生效；日常 P4 Client 不运行 sync、reconcile 或 revert。
4. 从失败节点继续、修订续跑或复用成果时沿用已记录的基线与来源快照，不重新切到最新提交。
5. 仅平台适配器调用 P4；Agent 不取得修改模板 Client 或直接提交 depot 的能力。

## 结果交付

运行结果页增加“生成待审核 changelist”，针对已结束且有代码产物的运行，从记录的最终检查点导出精确的增加、修改和删除清单。在平台拥有的独立 Client 中创建编号 changelist，并 shelve 供审查或取回；不自动执行 `p4 submit`。

交付记录保存运行、基线、检查点、Client、changelist 号、状态及错误。请求幂等，重试复用同一记录和编号，避免重复建单。只处理运行差异中的文件，不扫描或吸纳其他改动。无差异时明确提示无需生成。

失败交付保留错误和可重试入口。已经产生的待审核 changelist／shelf 不由普通运行过期清理自动删除；其关联 Client 和文件保留策略须明确检查，避免清理后无法继续审查。

## 实施与验收

分两部分落地：先完成模型选项并验证真实命令参数，再完成 Perforce 来源、隔离工作区和交付。测试先行，修改函数控制在 70 行内。

模型测试覆盖目录能力解析、非法组合、保存和发布、固定版本快照、Fast 显式关闭，以及原生执行和 Eino 桥接参数传递。页面验证控件可见、切换模型提示准确。

Perforce 测试覆盖登记字段、路径越界、模板映射、固定基线、增改删导出、失败续跑、交付幂等及保留期。自动测试使用受控 CLI 测试替身和临时目录；真实 P4 验证使用专用测试 Client／范围，不能在日常项目中造测试改动。

完成后执行相关 Go 与前端测试、契约检查、构建、OCR delegate 文件清单和规则解析，再由当前代理审查本次改动。部署前检查在途任务，验证 Git 项目正常，记录 Perforce 验证范围与环境限制。

## 依据

模型选项依据本地固定 Codex 源码 `.tools/codex-runtime/codex-rs/core/src/config/mod.rs` 和实际运行程序 `debug models --bundled` 的返回值。

Perforce 的 Client 用于定义独立映射，reconcile 将增加／修改／删除归入指定待提交清单，shelve 将待提交内容保存供审查：[Client 文档](https://help.perforce.com/helix-core/server-apps/cmdref/current/Content/CmdRef/p4_client.html)、[reconcile 文档](https://help.perforce.com/helix-core/server-apps/cmdref/current/Content/CmdRef/p4_reconcile.html)、[shelve 文档](https://help.perforce.com/helix-core/server-apps/cmdref/current/Content/CmdRef/p4_shelve.html)。
