# Pangu 原生 Codex 行为追踪构建

基于 OpenAI Codex rust-v0.163.0，固定提交 83086188ae0614bd882818ff1aa23f3194962012。保留原生模型连接、沙箱、登录和工具实现，增加模型与工具边界上的结构化事件。Pangu 仍调用 codex exec。

- 取消工具计数器、30 秒 watchdog、Pangu CLI 总时限。恢复原生长命令会话和 write_stdin；每次真实调用分别记录。
- 模型请求记录模型名、输入条目/工具数、结束状态和 token 摘要；不输出 reasoning 内容。工具记录实际参数、结果和耗时，通过原生 call_id 关联发起模型。
- 输出协议为单行 pangu.trace；Go 端为每个 CLI 会话加唯一 span 前缀，并以受信任上下文绑定运行、节点和尝试。入库前脱敏，正文过长有截断标记。追踪写入失败停止后续执行。
- 保留直接工具模式，关闭托管工具、hooks、legacy notify、个人插件、子 Agent 和自动安装。项目沙箱与专用登录隔离继续生效。
- 产品经理搜索仅允许 web.run，参数固定为用户授权词；不能借搜索调用其他原生工具。Eino 模型桥通过 PANGU_CODEX_MODEL_ONLY 显式禁止原生工具，由 Eino 工具集执行工具请求。
- 权限拒绝使用退出码 87，追踪管道故障使用 88。Windows Job Object / 进程组仍负责主动取消与后代终止，远端副作用不能靠本地取消回滚。

Worker 设置 PANGU_CODEX_TRACE=1；PANGU_CODEX_TRACE_PROBE=1 返回 pangu-codex-trace/1。这是能力检查，不是对恶意可执行文件的签名认证。旧受控构建和普通 CLI 均不满足新协议。

执行 scripts/codex-build.ps1 构建到 .tools/codex/codex.exe，不覆盖系统 Codex、不复用个人登录。脚本验证固定源码、补丁、锁文件与 policy.rs，产出源码/补丁/二进制摘要清单，打包拒绝过时构建。上游采用 Apache-2.0；源码留在 .tools，主仓库维护 native.patch 和 policy.rs。

测试使用真实 CLI 对本地 SSE 模拟端点，覆盖超过 10 次工具实际写入、父子 trace、超过 30 秒命令、主动取消和只读沙箱；启用 PANGU_CODEX_CONFIG_CHECK=1 并指定受控程序路径，不调用外部模型。业务模型质量和正式部署验收另行记录。
