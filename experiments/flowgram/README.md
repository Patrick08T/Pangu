# FlowGram 接入实验

独立实验，不属于 Pangu 生产构建，不连接数据库、Hatchet、模型账号或项目文件。使用虚构的 Eino → Codex 流程验证画布与 Pangu 配置的边界。

## 复现

在本目录执行：

```powershell
npm.cmd ci --ignore-scripts
npm.cmd test
npm.cmd run build
npm.cmd run dev
```

打开 `http://127.0.0.1:8082`。默认与 Pangu 一样启用 React StrictMode。若出现初始化错误，页面链接可切到 `?strict=off`，仅用于对照实验。

## 2026-10-26 结果

- 固定版本：FlowGram 1.0.16、React / React DOM 19.3.1、Vite 8.3.5。
- 四项数据测试通过：完整样本往返、移动与断线、删除保留失效绑定、拒绝未知/重复节点及悬空边。
- `npm run build` 成功。最小示例 JavaScript 为 967.56 kB（包含 React 和实验错误边界，gzip 291.94 kB）；不是与 Pangu 完整包的同口径性能比较。
- Chrome 开发环境启用 StrictMode 时，初始化报 `Ambiguous match found for serviceIdentifier: FlowRendererRegistry`。错误边界显示该信息，画布无法渲染。
- 同一页面仅关闭 StrictMode 后可以渲染两节点、连线，并通过实际 `document.toJSON()` → Pangu → `document.fromJSON()` → 再导出的检查。
- 未证明关闭 StrictMode 是合适的正式修复，也未覆盖完整生产编辑行为、鼠标拖拽或 FlowGram 表单/变量引擎。此结果不代表 FlowGram 在所有 React 19 环境不可用。

## 决策

本轮不替换生产画布。开发环境兼容门槛尚未通过；当前 Pangu 已有固定版本、字段结构、直接上游引用范围和草稿撤销机制，简单引入画布并不能替代这些产品约束。保留现有 React Flow，交付助手表单、变量引用及调试改进。

重新评估条件：上游提供可在 Pangu StrictMode 环境验证的修复，并证明接入表单/变量引擎能够减少维护工作。无需修改已有流程数据。

## 数据边界

`bridge.mjs` 仅用于已知样本：Pangu 原始对象保留在宿主，FlowGram 只提供节点位置和连线；条件、审批、预算、绑定及固定版本不交给编辑器运行时。删除节点会留下失效变量供 Pangu 校验。它不是公开导入接口或生产迁移工具，未处理全部草稿形态和所有非法图。

[官方加载与保存 API](https://flowgram.ai/guide/free-layout/load.html) · [项目与 MIT 许可](https://github.com/bytedance/flowgram.ai)
