import { apiRequest, APIError } from './request';
import {download} from './download';

afterEach(() => vi.unstubAllGlobals());

test('修改携带 CSRF 和固定幂等键，错误保留可查询的请求编号', async () => {
  const fetcher=vi.fn(async()=>new Response(JSON.stringify({error:'版本已变化'}),{status:409,headers:{'X-Request-ID':'request-1'}}));
  vi.stubGlobal('fetch',fetcher);
  await expect(apiRequest('/runs',{method:'POST',body:{input:'测试'},csrf:'csrf-value',key:'same-key'})).rejects.toMatchObject({status:409,requestID:'request-1'});
  const [url,options]=fetcher.mock.calls[0] as unknown as [string,RequestInit];
  expect(url).toBe('/api/runs');
  const headers=new Headers(options.headers);
  expect(headers.get('X-CSRF-Token')).toBe('csrf-value');
  expect(headers.get('Idempotency-Key')).toBe('same-key');
  expect(options.credentials).toBe('same-origin');
});

test('检查失败保留组件诊断，下载认证失败不生成文件',async()=>{
  const body={ready:false,checks:[{component:'worker',status:'unavailable'}]};
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify(body),{status:503})));
  await expect(apiRequest('/workflows/preflight',{method:'POST'})).rejects.toMatchObject({response:body});
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({error:'请重新登录'}),{status:401})));
  const listener=vi.fn();window.addEventListener('pangu:unauthenticated',listener);
  await expect(download('/runs/a/artifacts.zip','结果.zip')).rejects.toMatchObject({status:401});
  expect(listener).toHaveBeenCalledOnce();window.removeEventListener('pangu:unauthenticated',listener);
});

test('页面退出后取消下载，迟到正文不会触发保存',async()=>{
  const controller=new AbortController();let finish!:(blob:Blob)=>void;
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,status:200,blob:()=>new Promise<Blob>(resolve=>{finish=resolve;})})));
  const pending=download('/runs/a/artifacts.zip','结果.zip',controller.signal);
  await Promise.resolve();controller.abort();finish(new Blob(['内容']));
  await expect(pending).rejects.toMatchObject({name:'AbortError'});
});

test('拒绝非 JSON 错误页面，401 通知界面退出', async () => {
  const listener=vi.fn();window.addEventListener('pangu:unauthenticated',listener);
  vi.stubGlobal('fetch',vi.fn(async()=>new Response('<script>不可信页面</script>',{status:401})));
  try { await apiRequest('/runs');throw Error('应失败'); } catch(error) {
    expect(error).toBeInstanceOf(APIError);expect((error as Error).message).not.toContain('<script>');
  }
  expect(listener).toHaveBeenCalledTimes(1);window.removeEventListener('pangu:unauthenticated',listener);
});
