import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './environment/status';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function mockServer(database: 'ok' | 'unavailable') {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    const body = url.endsWith('/healthz') ? { status: 'ok' } : {
      status: database === 'ok' ? 'ready' : 'not_ready',
      scope: 'api_dependencies',
      checks: { database, hatchet_registration: 'ok' },
    };
    return new Response(JSON.stringify(body), {
      status: url.endsWith('/readyz') && database !== 'ok' ? 503 : 200,
      headers: { 'X-Request-ID': 'request-123' },
    });
  }));
}

test('依赖未就绪时展示原因，并可刷新到就绪状态', async () => {
  mockServer('unavailable');
  render(<App />);
  await screen.findByText('数据库未就绪');
  mockServer('ok');
  fireEvent.click(screen.getByRole('button', { name: '重新检查' }));
  await screen.findByText('基础依赖已就绪');
  expect(screen.getByText(/不代表 Worker 在线/)).toBeTruthy();
});

test('网络失败不会显示虚假就绪', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
  render(<App />);
  await screen.findByText('无法连接 API');
  await waitFor(() => expect(screen.queryByText('基础依赖已就绪')).toBeNull());
});

test('服务返回互相矛盾的状态时拒绝显示就绪', async () => {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => new Response(JSON.stringify(
    url.endsWith('/healthz') ? { status: 'ok' } : {
      status: 'ready', scope: 'api_dependencies',
      checks: { database: 'unavailable', hatchet_registration: 'ok' },
    },
  ))));
  render(<App />);
  await screen.findByText('无法连接 API');
  expect(screen.queryByText('基础依赖已就绪')).toBeNull();
});
