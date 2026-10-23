# Pangu 原生 Codex 受控构建

基于 OpenAI Codex `rust-v0.163.0`，固定提交 `83086188ae0614bd882818ff1aa23f3194962012`。保留原生 Agent、模型连接、沙箱、登录和工具实现；增加一个独立计数/时限模块及调用接入。Pangu 仍通过 `codex exec` 调用，不用提示词或事后观察来模拟执行前限制。

- 每次节点尝试启动独立进程。所有经过原生工具注册表的调用在执行前原子计数；失败调用也计数，超限不进入工具处理器。上限为节点策略的 0–10 次。
- 每个调用持有独立 30 秒计时器。超时直接退出 CLI，Pangu 等待整个 Job/进程组停止后才返回失败；失败工作区不会发布。
- 原生命令执行强制使用同步完成模式，不提供后台会话或 `write_stdin`，避免通过先返回会话绕开单次时限。
- 受控模式强制直接工具模式，模型目录不能重新开启异步 Code Mode。Pangu 同时禁用 hooks、清空 legacy notify，避免自动命令或 MCP hook 在注册表之外执行。
- 托管搜索、图像生成、个人插件、子 Agent 和自动技能安装由 Pangu 关闭。管理员显式注册的 MCP 仍经过工具注册表；远端服务自身的外部副作用不能靠本地进程终止回滚。
- CLI 进程总时限与节点时限继续有效。工具超限、工具超时不重试；恢复使用已完成检查点。预算按尝试计算，失败尝试中的外部模型调用可能重复计费。

策略违规使用原生退出码87、工具时限使用86，不能只返回上游 `Fatal`：固定版本会在工具收尾阶段记录该错误后继续请求模型。Pangu 保留原退出码和脱敏诊断，结束整个进程组后才返回不可重试失败。Windows 显式使用官方非提权沙箱，避免未配置时将 `workspace-write` 降为只读；只读模板仍使用 `read-only`。

`PANGU_CODEX_TOOL_CALLS` 仅由 Worker 生成，不接受用户直接注入。`PANGU_CODEX_POLICY_PROBE=1` 的探测返回协议标识 `pangu-codex-policy/1`；这是管理员所安装程序的能力检查，不是对恶意可执行文件的签名认证。普通未打补丁的 CLI 会被拒绝。

Windows 构建：安装 Git、Rust stable、MSVC C++ 编译工具后，在项目根目录执行 `./scripts/codex-build.ps1`。输出为 `.tools/codex/codex.exe`。脚本逐文件验证固定源码、补丁及锁文件，拒绝额外源码；产出源码/补丁/二进制摘要清单，打包拒绝过时或变更的二进制。不覆盖系统安装的 Codex，也不复用个人登录。当前本机编译器为 Rust 1.96.0。

独立计数测试使用 40 个线程争用 10 个名额。Go 的 `TestNativeCodexDeniesEleventhToolBeforeWriting` 与 `TestNativeCodexToolDeadlineStopsDescendants` 使用真实受控 CLI 和本地 SSE 服务验证真实文件写入、超限拒绝及超时后无后代写入；需设置 `PANGU_CODEX_CONFIG_CHECK=1` 和受控程序路径，不调用外部模型。真实模型质量与平台沙箱验收仍另行执行。

上游采用 Apache-2.0；原生补丁与构建产物须附带上游许可证。上游代码不复制进主仓库，`native.patch` 和 `policy.rs` 是 Pangu 的可复现变更。升级必须重新验证调用覆盖、后台进程、能力探测及上述集成测试，不能直接替换普通 CLI。
