import { useEffect, useState } from 'react';
import { readStatus, type ServiceStatus } from '../api/client';

function useServiceStatus(refresh: number) {
  const [state, setState] = useState<{ loading: boolean; error: boolean; data?: ServiceStatus }>({ loading: true, error: false });
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 8000);
    setState({ loading: true, error: false });
    readStatus(controller.signal).then(
      (data) => { if (active) setState({ loading: false, error: false, data }); },
      () => { if (active) setState({ loading: false, error: true }); },
    ).finally(() => clearTimeout(timeout));
    return () => { active = false; clearTimeout(timeout); controller.abort(); };
  }, [refresh]);
  return state;
}

function ServiceCard({ title, value, description, good }: { title: string; value: string; description: string; good?: boolean }) {
  return <section className="service-card">
    <p className="card-label">{title}</p>
    <h2><span className={`dot ${good === undefined ? 'neutral' : good ? 'good' : 'warning'}`} />{value}</h2>
    <p>{description}</p>
  </section>;
}

export default function App() {
  const [refresh, setRefresh] = useState(0);
  const { loading, error, data } = useServiceStatus(refresh);
  const checks = data?.readiness.checks;
  const title = loading ? '正在检查服务' : error ? '无法连接 API' : data?.readiness.status === 'ready' ? '基础依赖已就绪' : '部分依赖尚未就绪';
  const unknown = loading ? '检查中' : '未获取状态';
  return <>
      <header><div><p className="eyebrow">工作空间 / 概览</p><h1>运行环境</h1><p className="intro">开始任务前，检查服务连接与基础依赖。</p></div><button disabled={loading} onClick={() => setRefresh((value) => value + 1)}>{loading ? '检查中…' : '重新检查'}</button></header>
      <div className={`summary ${error ? 'error' : ''}`} role="status" aria-live="polite"><strong>{title}</strong><span>{error ? '请启动本地 Pangu API，确认端口为 8081 后重新检查。' : '结果仅对应本次检查，不代表 Worker 在线、模型可用或任务可以执行。'}</span></div>
      <div className="services">
        <ServiceCard title="Pangu API" value={data ? '进程在线' : unknown} good={data ? true : undefined} description="提供编排校验、任务提交和运行查询。" />
        <ServiceCard title="产品数据库" value={checks ? checks.database === 'ok' ? '数据库已就绪' : '数据库未就绪' : unknown} good={checks ? checks.database === 'ok' : undefined} description="检查独立 PostgreSQL 连接与迁移版本。" />
        <ServiceCard title="Hatchet 调度" value={checks ? checks.hatchet_registration === 'ok' ? '编排已注册' : '连接或注册未就绪' : unknown} good={checks ? checks.hatchet_registration === 'ok' : undefined} description="检查调度服务连接及编排注册状态。" />
      </div>
      <section className="next-step"><div><h2>准备好后，直接开始任务</h2><p>选择已发布的助手，填写目标即可。系统会在执行前检查当前项目、Worker 和模型配置。</p></div><a className="button primary" href="#/onboarding">新建任务</a></section>
      <section><h2>按缺项完成配置</h2><ul><li><a href="#/projects">项目与成员</a>：登记 Git 来源、分支和执行服务。</li><li><a href="#/connection">模型连接</a>：选择模型 API 或 Worker 专用 Codex 账号；模型密钥由维护者在服务端配置。</li><li><a href="#/agent">助手</a>：创建并发布助手后，就能在新建任务中选择。</li><li>联网搜索：复用 Worker 专用 Codex 账号的原生搜索，无需额外搜索密钥。请保持 Codex 安装和登录有效；不要将私密资料填入公开搜索词。</li></ul></section>
      <details><summary>开发者接口</summary><a className="contract-link" href="/api/openapi.yaml">查看 API 契约</a></details>
      <footer><span>本地服务 · 仅供受信用户使用</span>{data?.requestID && <span className="request-id">检查编号：{data.requestID}</span>}</footer>

  </>;
}
