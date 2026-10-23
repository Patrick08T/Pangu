import type { components } from './schema';

export type Readiness = components['schemas']['Readiness'];
export type ServiceStatus = { readiness: Readiness; requestID: string };

export async function readStatus(signal: AbortSignal): Promise<ServiceStatus> {
  const [health, ready] = await Promise.all([
    fetch('/api/healthz', { signal, cache: 'no-store' }),
    fetch('/api/readyz', { signal, cache: 'no-store' }),
  ]);
  if (!health.ok || (!ready.ok && ready.status !== 503)) throw new Error('服务检查失败');
  const process: unknown = await health.json();
  const dependency: unknown = await ready.json();
  if (!process || typeof process !== 'object' || !('status' in process) || process.status !== 'ok') {
    throw new Error('进程状态响应无效');
  }
  if (!isReadiness(dependency)) throw new Error('依赖状态响应无效');
  if ((ready.status === 200) !== (dependency.status === 'ready')) throw new Error('依赖状态与 HTTP 状态不一致');
  return { readiness: dependency, requestID: ready.headers.get('X-Request-ID') ?? '' };
}

function isReadiness(value: unknown): value is Readiness {
  if (!value || typeof value !== 'object' || !('scope' in value) || value.scope !== 'api_dependencies') return false;
  if (!('status' in value) || !['ready', 'not_ready'].includes(String(value.status))) return false;
  if (!('checks' in value) || !value.checks || typeof value.checks !== 'object') return false;
  const checks = value.checks;
  if (!('database' in checks) || !('hatchet_registration' in checks)) return false;
  const values = [checks.database, checks.hatchet_registration];
  return values.every((status) => status === 'ok' || status === 'unavailable')
    && (value.status === 'ready') === values.every((status) => status === 'ok');
}
