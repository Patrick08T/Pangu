# 大型本地 P4 实施计划

已获用户设计确认；索引不动、不提交Git、不迁移产品DB、不发布当前混合工作树。

## 固定边界

- 实施依据：2026-11-12大型本地Perforce设计（含用户确认的时间/取消规则）。
- 基线只采用已复现旧源码与旧UI+先前20s/28s修复+明确审查白名单。
- 快照阶段无真实P4写入、无业务run、无模型调用；只有最终一次业务提交按原流程运行。
- 先观察行为RED，再最小实现GREEN；缺符号/语法错误不能作为RED。函数≤70行。
- 每名执行者只写分配范围，所有后台作业必须收集；Lead审核总diff、测试和安装指纹。

## 分步任务

1. **task-49 / memory-core：P4层（与task-50并行）**
   - 先测试嵌套.git的精确排除与root/delivery严格拒绝，保留合法普通文件。
   - 新LocalInventory报告API，保持LocalFiles兼容；一次不可变命令session。
   - 分阶段和总预算、零输出慢扫描、晚错/资源边界/取消行为。
   - CL归属不变的有界TrackLocal批处理，脱离全量opened的普通8MiB瓶颈。
   - 只写internal/perforce和新artifact/p4-tests，离线fake进程与临时文件系统。

2. **task-50 / evaluation-core：纯本地Git层（与task-49并行）**
   - 只新增internal/workspace/local_git前缀文件和测试，不修改已有接线。
   - manifest严格校验；raw-byte fast-import；受控进程/有界NUL流；树完整性检查。
   - 本地bulk命令与空间准入/运行中保留余量；普通Git行为不改。
   - 真临时Git测试无gitlink/filters、同mtime+size字节变化、取消、>16MiB及无变更二次捕获。

3. **Lead：生命周期接线（依赖49、50接口完成）**
   - 在新代码完成前准备现有消费者和测试DB的独立验证环境，不触产品DB。
   - localSnapshot使用inventory→manifest→raw import→前后绑定复核；总2h，初次额外容量准入。
   - 本地P4新运行单对象库发布到base，不checkout；旧source/base可读可恢复；普通Git/隔离P4不变。
   - 本地rerun/fork独立对象拷贝、无checkout，保留Lease和父提交/HEAD验证。
   - 内部变化路径流用于安全跟踪，UI/export保持原限制；finish/recovery独立2h且不绕过停止确认。
   - 回归ready/成功head/失败local_commit/p4_tracked门槛、原CL归属、旧布局和fork隔离。

4. **Lead：构建与审查**
   - 先比对原索引与安装/源码基线指纹。
   - 在新artifact建立只读白名单派生overlay，不覆盖昨晚候选/证据。
   - 跑perforce、workspace、scheduler、api、architecture及测试DB相关回归；独立审查修复到最终指纹。
   - 测试数据库与产品数据库隔离；没有测试条件时明确SKIP，不伪称通过。

5. **Lead：真实无业务演练**
   - 一次完整LocalInventory，统计全部阶段、被排除metadata、regular/missing/special、P/N/S/最大文件。
   - 不保存全量文件内容或文件名到日志，不将不完整reconcile解释为零新文件。
   - 空间准入通过后，仅在Worker管理的独立演练目录完整捕获与无修改二次捕获。
   - 测量峰值RAM、卷/对象/暂存占用、耗时，验证树无gitlink、取消/恢复与head完整性。
   - 碰到已知门槛失败先修复或报告，不消耗唯一业务重提次数。

6. **Lead：安全发布**
   - 确认无冲突中的业务写任务；备份原安装文件/日志/进程信息。
   - 仅同路径替换普通权限Pangu文件；不改ACL、Codex、Hatchet、数据库或用户P4文件。
   - API/worker健康不通过自动恢复旧二进制；不回退用户内容或不可变证据。

7. **Lead：一次业务验证**
   - 选择Partopia 23575b1c-13c4-438b-ae99-42bafa232b08。
   - 工作流v2 cc68adec-c6ce-45e7-b448-f7f8ba2cc6e4、原GM/spawner中文输入、原implement人工审批。
   - 仅一次点击；核实基线与节点启动；不批准用户保留的人工实现节点，不自动再次提交。

## 冻结接口

```go
// perforce
func (s Snapshot) LocalInventory(ctx context.Context) (LocalInventory, error)
type LocalInventory struct {
    Files []string
    PathBytes int64
    ExcludedMetadata int
    Stages []LocalInventoryStage
}
type LocalInventoryStage struct { Name string; Records int; Duration time.Duration }

// workspace；独立纯本地辅助，不含DB/P4调用
func inspectLocalFiles(ctx context.Context, root string, names []string) (localManifest, error)
func importLocalFiles(ctx context.Context, repository, root, parent string, manifest localManifest) (string, error)
func inspectLocalRepository(ctx context.Context, repository, commit string) error
func streamLocalGit(ctx context.Context, repository string, args []string, visit func([]byte) error) error
func runLocalGit(ctx context.Context, directory string, args ...string) (string, error)
func checkLocalCaptureCapacity(ctx context.Context, repository string, manifest localManifest) error
func localVolumeSpace(path string) (free, total uint64, err error)

type localManifest struct {
    Files []localFileInfo
    TotalBytes int64
    PathBytes int64
    Missing int
    Largest int64
}
// localFileInfo: Name string, Size int64, Mode fs.FileMode, ModTime time.Time；可加私有身份字段。
```

importLocalFiles接受合法已有parent，成功保留独立local-UUID引用但不改HEAD；初次2.4*S容量准入由Lead显式调用，运行中每次导入持续维护R余量。任何接口调整先协商，防止并行实现漂移。

## 交付记录

每步记录命令、RED/GREEN结果、最终指纹、真实测量与明确未测项。旧artifact不可变；新证据放入20261112独立目录。所有阶段未完成前不得将目标标记完成。
