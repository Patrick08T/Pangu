export class APIError extends Error {
  constructor(message: string, readonly status = 0, readonly requestID = '', readonly response?: unknown) { super(message); }
}

type Options = { method?: string; body?: unknown; csrf?: string; key?: string; signal?: AbortSignal };

export async function apiRequest<T>(path: string, options: Options = {}): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' });
  if (options.body !== undefined) headers.set('Content-Type', 'application/json');
  if (options.csrf) headers.set('X-CSRF-Token', options.csrf);
  if (options.key) headers.set('Idempotency-Key', options.key);
  let response: Response;
  try {
    response = await fetch(`/api${path}`, { method: options.method ?? 'GET', headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: 'same-origin', cache: 'no-store', signal: options.signal });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new APIError('连接中断，操作结果可能尚未返回。请用相同输入重试，或查看运行记录。');
  }
  if (response.status === 204) return undefined as T;
  const value: unknown = await response.json().catch(() => undefined);
  if (response.status === 401) window.dispatchEvent(new Event('pangu:unauthenticated'));
  if (!response.ok) {
    const message = value && typeof value === 'object' && 'error' in value && typeof value.error === 'string'
      ? value.error : `服务请求失败（${response.status}），请稍后重试。`;
    throw new APIError(message, response.status, response.headers.get('X-Request-ID') ?? '',value);
  }
  if (value === undefined) throw new APIError('服务回执格式无效，操作结果尚未确认。请保留原输入重试。',0,response.headers.get('X-Request-ID')??'');
  return value as T;
}
