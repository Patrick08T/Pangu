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
      <section className="next-step"><div><p className="eyebrow">开始使用</p><h2>通过 API 配置与运行工作流</h2><p>先登记项目，再发布并绑定模板。执行前检查可验证 Worker 与配置，不会发起收费模型请求。</p></div><a className="contract-link" href="/api/openapi.yaml">查看 API 契约 <span aria-hidden="true">↗</span></a></section>
      <footer><span>本地服务 · 仅供受信用户使用</span>{data?.requestID && <span className="request-id">检查编号：{data.requestID}</span>}</footer>

  </>;
}
