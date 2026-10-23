# Pangu

基于 **Go + Hatchet + Eino + 原生 Codex** 的 Agent 工厂，提供中文 React 控制台和 JSON/API 编排。每个 Agent 节点由 Hatchet 独立调度、重试和恢复；产品配置、版本、身份与审计使用独立 PostgreSQL 数据库和账号。

已实现共享模板、不可迁移的项目绑定、不可变版本、串并行 DAG、定时任务、运行与产物查询、飞书权限、审计及保留清理。准确状态见 [实施进度](docs/实施进度.md)，验收项见 [46 项工程任务](docs/工程任务卡.md)。真实飞书、模型、团队采纳率及干净机器安装验收尚未全部完成，本地模拟验证不代表完整产品验收。

## 启动与配置

开发构建需要 Go 1.26.7+、Node.js 24；本机验证 Go 1.26.10。安装 ZIP 含前端和服务程序，使用时不需要 Go/Node。详细安装、升级、启停及备份恢复步骤见 [运行与恢复](docs/运行与恢复.md)。

```powershell
npm.cmd ci --prefix web
npm.cmd run build --prefix web
& ./.tools/go/bin/go.exe build -o .tools/pangu.exe ./cmd/pangu
./scripts/hatchet.ps1 -Background
```

等待 `.tools/hatchet-connection.json` 生成。首次建库按手册临时提供管理员连接并执行 `scripts/database-init.ps1`，服务进程不携带管理员凭据。

```powershell
. ./scripts/hatchet-env.ps1
. ./scripts/database-env.ps1
& ./.tools/pangu.exe -mode migrate
# 在当前终端安全配置飞书、模型及 Worker 参数
./scripts/start.ps1 -Port 8081
```

访问 `http://127.0.0.1:8081/`；前端由 Go 同源托管，API 前缀为 `/api`。前端开发用 `npm.cmd run dev --prefix web`，5180 代理到 8081。程序不会自动加载 [.env.example](.env.example)，真实凭据不得提交。

停止依次使用 `scripts/stop.ps1`、`scripts/hatchet-stop.ps1`。Hatchet 重启会更换本地令牌和 PostgreSQL 端口，应重新加载连接并重启 Pangu。`-mode api` 和 `-mode worker` 可拆开进程。

飞书需配置应用、允许租户、公开同源地址与显式首位管理员 open_id；未配置时受保护接口返回 503，不自动提升首位访客。项目角色为管理者、运行者、查看者，API Token 另受项目/操作范围限制。详见 [飞书与项目权限](docs/飞书与项目权限.md)。

## 项目与 Agent

先登记 Worker 上已有的本地 Git 项目、分支与稳定 Worker ID。运行在 `PANGU_WORKSPACE_ROOT` 下创建独立 clone、固定基准提交；每次尝试隔离，成功才发布检查点。失败写入不污染重试或源项目，不自动提交/推送源仓库。

共享模板可跨项目复用，绑定为项目 Agent 后不能迁移项目。修改草稿不影响发布版本或已接受运行；升级必须显式发布并重新引用版本。配置每次从 PostgreSQL/环境变量原始来源读取，不做进程内缓存。

Eino 产品经理支持调研、竞品分析、产品规划、方案设计四种模式；受控工具包含公开搜索、网页、原始项目资料、直接上游报告和报告登记。来源、输入输出和设计任务依赖均校验。公开搜索使用管理员提供的 Brave Search API。详见 [模板与产品经理](docs/模板与产品经理使用.md)。

Codex 使用独立 `PANGU_CODEX_HOME` 和 `PANGU_CODEX_BINARY`，由管理员在专用目录登录；个人配置、额外环境密钥及自动插件不会继承。写入/只读模板使用对应原生沙箱。

工具限制通过固定源码的 [Codex 受控构建](runtime/codex/README.md) 实现，普通 CLI 缺少必需能力时拒绝运行。用 `scripts/codex-build.ps1` 构建，不覆盖系统 Codex。每次尝试最多 10 次工具调用、单次 30 秒；超限在执行前拒绝，超时停止受控进程。真实构建验收状态以实施进度为准。

## API 与执行边界

完整接口和类型见 [OpenAPI](api/openapi.yaml)、[运行契约](specs/运行契约.md)。核心入口包括运行、请求回执、配置对象/版本、Cron、项目/成员、产物、Token 和审计。

提交必须携带项目和唯一 `Idempotency-Key`。202 表示意图已持久化；`run_id` 尚未返回时按 `Location` 查询。相同键/内容返回原回执，改变内容返回 409。列表使用产品记录分页；远端暂未同步的项显示 `unknown`，不导致整页失败。返回产品 DTO，不暴露 Hatchet 原始载荷。

- 最多 32 节点、256 KiB 请求；带上游输出的单节点输入最多 1 MiB，输出最多 16 KiB。支持无环串并行，首版没有条件分支、循环或拖拽流程设计器。
- 最多重试 2 次；Eino 节点/模型为 3/2 分钟，Codex 节点/CLI 为 30/25 分钟，运行/排队各 6 小时。参数、认证、验证命令失败与工具超限不盲目重试。
- Cron 使用 UTC 五字段、固定版本、同分钟防重、活动运行不重叠，停机漏跑不补。暂停/删除不取消已接受运行。
- Windows 使用 Job Object 确认整个进程组停止。成功检查点可恢复；外部模型调用和远端工具副作用不保证恰好一次。
- 临时输入、日志、报告、工作区与重跑基准保留 30 天；元数据/幂等记录 90 天；审计/清理记录 180 天。活动运行和下载租约受保护，过期明确返回 410。
- Hatchet core/OLAP 使用 720h 分区保留；Pangu 每分钟归档确认停止后的无正文摘要，远端删除后仍可查询历史。长期停机可能造成尚未归档的远端终态丢失，需要持续运行归档器与备份。
- Pangu 仅监听回环；远程部署仍需 TLS 和运维配置。embedded REST 的监听/防火墙由部署者核对。本地单机不提供高可用或敌对租户文件隔离。

## 验证与交付

```powershell
./scripts/check.ps1 -Database
# 真实 Hatchet、独立测试库；模型使用本地模拟服务
. ./scripts/hatchet-env.ps1
. ./scripts/database-env.ps1 -TestDatabase
$env:PANGU_INTEGRATION='1'
& ./.tools/go/bin/go.exe test ./internal/scheduler -count=1 -timeout=15m
```

真实 CLI 对本地端点的限制测试使用 `PANGU_CODEX_CONFIG_CHECK=1`。付费模型验收另用 `PANGU_CODEX_REAL=1`，会使用账号额度；模拟结果不记为用户采纳。

安装包用 `scripts/package.ps1 -Destination <绝对ZIP路径>` 生成；冷备份与全新目录恢复用 `scripts/backup.ps1`、`scripts/restore.ps1`。备份含敏感资料，必须存于受控目录。

Hatchet SDK/embedded 0.110.5（MIT，可完全本地自托管），Eino 0.9.22（Apache-2.0），Codex 基于 0.163.0（Apache-2.0）。Pangu 沿用 [AGPL-3.0](LICENSE)。

全部文件只用 Git，已豁免 P4。采用 TDD，Go 函数不超过 70 行，由 Codex 原生 Agent 审查。详见 [需求确认](docs/需求确认记录.md)、[架构设计](docs/架构设计.md)、[历史验证](docs/验证记录.md)。
