// Package experience 提供跨任务经验的授权存储、不可变水位、选择记录与依赖证明。
//
// 本包不运行模型、不调度学习、不解释人类审批、不自行发布版本。Authorize、
// VerifySource、VerifyClaim、VerifyBenefit 是服务端可信适配器，不是客户端布尔值。
// 适配器必须使用传入的 pgx.Tx，从原始权限/证据记录验证并持有相应事务锁；
// 如授权可被并发撤销，撤销路径必须使用同一锁协议。不可在回调中另开连接
// 然后把非事务读取误认为副作用提交前的授权复核。
//
// Scope 的 namespace/project/actor 均须由可信调用边界构造。水位与正文的读取
// 同样需要授权，不允许以“只返回哈希/摘要”为由绕过来源权限。Content 的 JSON
// 编码是 BodyHash 的 SHA-256 输入；Sources 与元数据由数据库不可变约束保护。
//
// 提交接收时须在 submission 的现有事务内调用 FreezeTx。runID 可使用预生成
// 的 submission request ID，后续所有轮次必须保持同一水位身份，不能重新查询
// 当前活动经验代替冻结集合。SelectionInput.CanaryScope 也用于 serving 阶段：
// 它是已经授权的任务范围标签，不得由调用者从 policy 自动选取一个匹配标签。
//
// CheckArtifact 验证已记录的依赖链和期望输出哈希；复用调用方仍须比较当前有效
// 输入指纹与 Artifact.InputHash。ProofVersion=0、缺失父证明和有环证明均不可复用。
// 即使后台失效标记尚未写回，读取时的递归验证也会阻止撤销来源的成果被继续使用。
package experience
