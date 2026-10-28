# Eino + Codex 性能分析示例

## 识别接入方式
检查 go.mod、节点构建与适配代码。
- 调用完整 Codex CLI/SDK：让其拥有分析任务内的探索循环。
- Eino 直接调用底层模型 API：可让 Eino 拥有工具循环。
- 父子 Agent：明确子任务输入、输出、上下文、截止时间及共享预算，避免两层重复管理整份报告。

不要混淆 Eino Workflow、Graph、ADK Workflow Agent 的 API 与循环支持，按安装版本验证。
不要假定 Eino checkpoint 自动保存 Codex 内部状态；保存可用线程/任务标识，测试恢复、取消、重复执行。

## 最小起点
1. Go/Eino 数据准备：固定报告、窗口、实例、部署版本，整理基础指标和证据索引。
2. 一个 Codex 分析节点：建立假设，用获准的 report_evidence 工具取证，输出发现、证据、反证和缺口。
3. Go 校验：schema、引用、范围、预算和必查项；仅在需要时增加语义审核。
4. 有限修正：传具体缺陷、复用查询账本和剩余额度，避免重新启动整次调查。
5. 业务服务保存报告、引用、版本和执行状态。

## 场景预算
“12 分钟、24 次取证”是该类场景的示例值，不是通用默认。沿用当前明确要求；不确定时标记待确认，不自动用于新项目。
工具入口执行原子计数，定义重试/缓存计数，为汇总留时间。到限输出部分结果，不由 Agent 擅自增加上限。

## 验收示例
| 情况 | 期望 |
|---|---|
| CPU/慢帧上升，缺 GC 数据 | GC 仍是待验证假设 |
| pprof 与源码版本不符 | 查正确版本；无法取得则限制结论 |
| 无显著异常 | 输出正常观察与检查范围 |
| 取证暂时失败 | 有界重试并记缺口，不把失败当零值 |
| 重复查同一快照 | 按范围和新鲜度复用，计数可追溯 |
| 达到预算 | 停新增取证，汇总支持结论和未决项 |
| 审核要求重查 | 返回具体缺陷，沿用账本和剩余预算 |

这些是设计案例；未运行不得报告已通过。

## 官方接口
- https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_collaboration/
- https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/
- https://developers.openai.com/codex/noninteractive
- https://developers.openai.com/codex/sdk

按安装版本核对结构化输出、事件流、错误传播、线程恢复、进程清理。不要将文档示例当生产实测。
