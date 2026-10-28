# Pangu 内置技能资源

## Workflow 构建与优化

- 标识：`build-optimize-workflow`。
- 来源：用户提供的 `C:\Users\pengtaotao\Downloads\build-optimize-workflow.zip`。
- 安装日期：2026-10-27。
- 原压缩包 SHA-256：`73E5D5A9C3896C6BDB79BE20F83592256BC11CA1521AA8B9FD4FEB23D79AA17E`。
- 本目录保留压缩包中的全部 7 个文件，未修改原文；本机 Codex 安装副本逐文件摘要一致。

技能提供构建、诊断、交接、预算与评估的方法。`assets/workflow-spec.yaml` 是设计模板，不是 Pangu 或 Eino 可以直接执行的工作流配置。

当前状态：已通过 Go embed 接入对话式工厂，随服务二进制发布。每轮按构建、修改、诊断或优化模式加载资料，并记录 SHA-256；部署不依赖个人技能目录。生成助手使用项目模型连接，通过 Eino 接入 Codex 账号或已授权 API，每轮最多调用模型一次，不调用业务工具；候选由领域校验和用户确认后批量发布。详见[使用与验收](../docs/对话式Agent工厂使用与验收.md)。

业务 Agent 配置仍以 PostgreSQL 为原始来源，每次读取重新加载。本技能的查询复用建议不得被解释为缓存可变配置。
