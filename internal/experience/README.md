# 经验基础服务（默认不启用）

这是经验存储、不可变版本、水位选择、成果依赖与学习意图的基础，不代表提炼调度、真实验证、收益评测、发布或生产入口已经接通。

## 可信边界

- `Store` 的 `Authorize`、`VerifySource`、`VerifyIntent`、`VerifyClaim`、`VerifyBenefit` 必须由服务端适配器提供；相关操作缺少回调即拒绝。客户端标志、来源摘要与模型自报结论不是授权或验证证据。
- 回调收到当前 `pgx.Tx`，必须从原始数据重读归属、权限、哈希、保留与证据状态。身份锁在经验锁之前；所有相关撤权/保留变更也必须遵守适配器的锁协议。
- 经验锁为 `experience:<namespace>:<projectID>`，使用事务级 advisory lock。内部事务操作在提交前再次调用授权回调。
- 不在进程中缓存正文；每次读取重新验证来源和完整正文哈希。正文删除后保留不可变、最小化的版本及审计元数据。
- `ReadTx`、`FreezeTx` 不提交调用方事务。`FreezeEmptyTx` 是受控匿名/内部任务的常量空水位接口：调用方必须持有身份锁、传入服务端确定的 purpose；它不调用身份回调、不读取版本正文、绝不返回已有非空水位。

## 接口要点

所有一般入口带 `context.Context` 和 `Scope{Namespace,ProjectID,ActorID}`。

| 接口 | 约束 |
| --- | --- |
| `Policy` / `SavePolicy` | 默认全关；append-only 修订与 expected revision CAS；启用字段不可推断默认阈值 |
| `Create` / `Read` / `ReadTx` | 强制 candidate+shadow；Content 完整 JSON ≤2048 bytes，必要类型、条件、反例、来源与适用角色必须具备 |
| `Activate` | 当前意图代际、策略、原始终态及独立主张验证全部成立；验证回调可在同事务写机器回执 |
| `SetDelivery` / `Revoke` | 晋级需可信收益检查；serving 需先经 canary；撤销删除正文并使后续读取/依赖立即失效 |
| `Freeze` / `FreezeTx` / `ReadWatermark` | 使用提交 requestID 作为稳定 RunID；固定版本、正文哈希和投放修订；禁止时间戳近似水位 |
| `Select` | 身份含运行/节点/轮次/审批代际/审批修订/输入指纹；首次选择落库，重试不重排；最多4条、总计≤8192 bytes |
| `ValidateReferences` | 核对真实当前来源、正文、有效性和历史获准投放修订，不用于候选影子实验的读取 |
| `SaveArtifact` / `ReadArtifact` / `CheckArtifact` | 不可变成果证明；递归检查直接经验与上游成果，缺证明/循环/撤销失败关闭 |
| `CheckArtifactInput` | 在上述检查外比较调用方提供的当前有效输入指纹 |
| `EnsureLearningIntent` / `CompleteLearningIntent` / `RetryLearningIntent` | 来源/证据修订/策略修订唯一，持久代际、幂等键与回执；旧代际不能完成新意图 |

`SelectionInput.Generation` 是当前执行/审批代际，`WatermarkGeneration` 是原任务冻结代际。后者为0时兼容前者；集成方应显式填原水位代际。新代际不得通过重新冻结原任务扩大授权版本集合。

`CanaryScope` 对 serving 和 canary 都是**服务端授权标签**，不可从客户端任意值或策略首项推断。两者都遵守确定性流量上限；当前策略收紧优先于旧投放快照。

`Policy.Enabled=false` 停止新学习、新晋级、新任务候选冻结，不破坏仍获准读取的既有水位。`ReadEnabled=false` 或 `Revoked=true` 撤销既有读取与相关成果复用。UI 应分别展示“自动学习/进化总开关”和“允许运行读取经验”。

`CheckArtifact` 不凭空知道调用方当前节点输入或审批许可；复用方仍须先执行当前选择/授权，并使用 `CheckArtifactInput` 比较当前有效输入。模型停机、取消确认、审批失效传播与系统重建由调度/审批层负责。

## 安全摘要与受控匿名运行

`ListVersions(ctx,scope,Page{AfterID,Limit})` 返回 `VersionPage{Items,NextCursor}`，摘要仅 ID、EntryID、Revision、Validity、Delivery、DeliveryRevision、ExpiresAt、CreatedAt。`Limit` 必须为1..100，版本游标为UUID。

`ListUsages(ctx,scope,versionID,page)` 返回 `UsagePage`，摘要仅 run_id、node_id、iteration、approval_generation、approval_revision、created_at；使用游标为64位小写hex的不透明分页标识，UI不展示。两种列表均同事务检查授权及每条来源，包括lookahead记录；来源拒绝则整页报错，不返回部分摘要或伪造空结果。正文已删除的墓碑可在原来源仍获授权时列出。没有输入、正文、正文哈希或来源字段，没有全表导出。RunID可能是提交requestID，未提供未经真实run映射验证的ListRunUsages。`Revoke` 按不可变versionID撤销，无revision-CAS承诺。

`NewFrozenEmptyStore(authorize Authorizer)` **仅用于可信调度器**，不是匿名HTTP访问入口。适配器必须在每个事务绑定服务器原始requestID、namespace/project、NULL created_by和原始空水位，并拒绝普通会话/token身份。内部不可导出的标志只允许空actor和 select/watermark_read/artifact_read/artifact_write/reference_read；非空actor、版本/策略操作、新建水位全部拒绝。SQL只读取既有空水位，不读策略正文；任何直接或祖先经验引用在来源/正文读取前拒绝。即使 `FreezeEmptyTx` 也不能通过该专用Store新建水位。通用Store仍拒绝空actor。

`Artifact.InputHash` 保持审批前逻辑dispatch输入指纹；`ExecutionInputHash` 另存真实resolved worker输入（含Revision/PlatformEvidence等）的指纹。`CheckArtifactInput` 仍比较前者。core兼容旧证明字段，scheduler必须对缺实际执行指纹的旧证明失败关闭，不能清空Revision来伪造相同输入。

## 真实运行审计

`ReadRunLineage(ctx,scope,RunLineageQuery)` 及 `ReadArtifactSummary(ctx,scope,id)` 使用 `lineage_read` 动作，仅当前具项目read权限的非空actor可调用；匿名runtime专用Store不允许审计。

`Store.VerifyRun` 类型为 `func(context.Context,pgx.Tx,Scope,RunLineageQuery)(RunBinding,error)`。运行审计缺此回调失败关闭。适配器必须同事务由 `ActualRunID` 查询原始submission/run记录，验证namespace/project、business purpose，并返回原 `RequestID` 和原始冻结代际；不得使用客户端request别名或max/latest代际。服务再次核对返回范围与查询条件。当前提交协议固定原水位gen=1，API适配器负责从可信协议确定，不由core猜测。

`RunLineageQuery` 包含 `ActualRunID`、可选可信 `RequestID/WatermarkGeneration`、必填 `UsagePage/ArtifactPage`（各Limit为1..100）。usage游标为64hex；artifact游标为原始ID，最多256 bytes。返回 `actual_run_id`、accepted watermark摘要、独立分页的usages/artifacts和 `chain_complete`。每页有 `total/has_more/next_cursor`；所有查询同时限定namespace/project与已验证的run/request别名。父成果可通过同项目 `ReadArtifactSummary` 逐级展开。

DTO见 [lineage_types.go](<lineage_types.go>)。仅返回白名单元数据、选择匹配理由枚举、注入引用hash、实际执行hash、refs和直接parents；不读取经验正文、模型输入或保护criteria。`status` 为valid/invalid/unproven/restricted；`proven` 标明证明可核验，不表示允许立即复用。失效证明仍能审计，不通过运行时 `ReadArtifact` 的有效性门禁。自由文本原因及来源回调细节不输出。

- 合法旧run没有水位：`present=false,status=unproven,invalid_reason=watermark_missing`，与已冻结空集合明确区分。
- 撤销/到期/撤读/祖先失效：固定reason `experience_revoked/experience_expired/policy_read_revoked/ancestor_invalid`，仍保留有权读取的审计链。
- 来源权限拒绝：`restricted/source_unavailable`；来源真实性失效：`invalid/source_invalidated`；来源缺失：`unproven/source_missing`。这些情况全部红删对应refs及所有关联hash，并传播至子成果；项目授权失败直接返回错误。未知数据库/回调故障也返回错误，不能伪装成失效或空页。
- 旧proof缺实际执行hash：`unproven/execution_input_missing`；缺祖先、循环或遍历边界也为unproven。没有artifact不推断链完整；只有当前首份全页包含全部已记录根和parents、没有红删或缺证明时 `chain_complete=true`，不宣称外部运行未落库的证明存在。

## 普通暂停与已确认安全阻断

普通delivery `paused` 只停止新选择；已冻结相同逻辑输入的精确重试不改变选择。`Enabled=false` 仍只停止新学习/新冻结，不等于撤销读权限；正常历史canary→serving引用保持有效。

仅可信机器证据writer可在delivery.policy的私有JSON写 `block_frozen:true`：已确认依赖/硬门禁失效，或完整可信cohort确认退化且需要主动阻断。样本不足、预算暂停、缺reader和普通总开关关闭不能写此标志。它不是public DTO字段，客户端不能宣称机器确认结论。

任何历史delivery存在该version的JSON boolean true，即粘性阻断旧usage精确重试、直接refs、祖先成果复用、新冻结和新选择；普通 `SetDelivery` 也不能覆盖清除。恢复须创建并独立验证新version。本安全门不改claim validity、不删除正文，不把退化伪装成主张错误；审计reason为 `frozen_safety_blocked`，与普通pause明确区分。所有检查仍在当前授权事务和经验锁内，SQL错误失败关闭。

## 测试与迁移

纯单测：

```powershell
& ./.tools/go/bin/go.exe test ./internal/experience -count=1
```

真实 PostgreSQL 契约测试（需要已迁移的独立无特权测试数据库；未设置 `PANGU_TEST_DATABASE_URL` 会失败，不会 Skip）：

```powershell
& ./.tools/go/bin/go.exe test -tags integration ./internal/experience -count=1
& ./.tools/go/bin/go.exe test -tags integration ./internal/experience -run 'TestStoreConcurrent|TestStoreCommit' -count=5
& ./.tools/go/bin/go.exe vet ./internal/experience
```

Windows 沙箱中应将 `GOCACHE`、`GOTMPDIR`、`TMP`、`TEMP` 指向工作区内获准目录；不要把环境错误当作业务 RED。测试不自动运行迁移，不连接生产/Hatchet 数据库。

[0056迁移](<../database/migrations/0056_experience.sql>)仅依赖此前项目表，不依赖后续迁移；已应用后不得修改 checksum。测试以受控可信回调验证本包契约，不替代真实身份、来源、评测适配器与端到端集成测试。仅提供上述受控摘要分页，没有公网原文检索或全表导出接口，不能绕过回调直接 SQL 查询未授权数据。
