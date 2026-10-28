import type { ReactNode } from 'react';
import { APIError } from '../api/request';
import type { components } from '../api/schema';

export type Schema = components['schemas'];
export type Session = Schema['Actor'];
export type Access = { session: Session; project: string; role: string; catalogManager?: boolean };
export const canEdit = (access: Access) => access.role === 'manager' || access.role === 'operator';
export const canManageCatalog = (access: Access) => access.session.admin || access.catalogManager === true || access.role === 'manager';
export const isManager = (access: Access) => access.role === 'manager';
export const timestamp = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—';

export function ErrorNotice({ error, retry }: { error?: Error; retry?: () => void }) {
  if (!error) return null;
  return <div className="notice error" role="alert"><strong>{error.message}</strong>
    {error instanceof APIError && error.requestID && <small>请求编号：{error.requestID}</small>}
    {retry && <button onClick={retry}>重试</button>}</div>;
}

export function Page({ title, description, actions, children }: { title: string; description?: string; actions?: ReactNode; children: ReactNode }) {
  return <><header className="page-heading"><div><h1>{title}</h1>{description && <p className="intro">{description}</p>}</div>{actions}</header>{children}</>;
}

export function QueryState({ loading, error, empty, retry }: { loading: boolean; error?: Error; empty?: string; retry?: () => void }) {
  return <>{loading && <p role="status">正在读取…</p>}<ErrorNotice error={error} retry={retry} />{!loading && !error && empty && <p className="empty">{empty}</p>}</>;
}

const statuses: Record<string, string> = { queued: '排队中', running: '执行中', cancelling: '取消中', cancelled: '已取消', succeeded: '已完成', failed: '失败', unknown: '结果未知', pending: '等待确认', accepted: '已接受', skipped: '已跳过', rejected: '已拒绝' };
export function Status({ value }: { value: string }) { return <span className={`status status-${value}`}>{statuses[value] ?? value}</span>; }

export function JSONField({ label, value, onChange, disabled, rows = 12 }: { label: string; value: string; onChange: (v: string) => void; disabled?: boolean; rows?: number }) {
  return <label className="field">{label}<textarea className="code-input" spellCheck={false} value={value} onChange={e => onChange(e.target.value)} disabled={disabled} rows={rows} /></label>;
}

export function Pager({ offset, next, change }: { offset: number; next?: number; change: (v: number) => void }) {
  return <div className="pager"><button disabled={offset === 0} onClick={() => change(0)}>第一页</button><span>当前位置 {offset}</span><button disabled={next === undefined} onClick={() => next !== undefined && change(next)}>下一页</button></div>;
}

export function ActionNotice({ busy, error, message }: { busy: boolean; error?: Error; message?: string }) {
  return <><ErrorNotice error={error} />{busy && <p role="status">正在处理…</p>}{message && <p className="notice" role="status">{message}</p>}</>;
}
