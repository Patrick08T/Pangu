import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Start} from './submission';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('结果未知时冻结输入并沿用同一个幂等键重试',async()=>{
  const keys:string[]=[];let calls=0;
  vi.stubGlobal('fetch',vi.fn(async(url:string,options:RequestInit)=>{
    if(url.includes('/versions/')&&options.method==='GET')return new Response(JSON.stringify({project_id:'00000000-0000-0000-0000-000000000001',kind:'agent'}));
    if(url.includes('/versions/')){keys.push(new Headers(options.headers).get('Idempotency-Key')!);if(++calls===1)throw new TypeError('offline');}
    return new Response(JSON.stringify({request_id:'00000000-0000-0000-0000-000000000003',project_id:'00000000-0000-0000-0000-000000000001',operation:'submit',status:'pending',created_at:new Date().toISOString()}),{status:202});
  }));
  render(<Start access={{session:{id:'user',name:'成员',admin:false,csrf:'csrf'},project:'00000000-0000-0000-0000-000000000001',role:'operator'}} version="00000000-0000-0000-0000-000000000002"/>);
  fireEvent.click(screen.getByRole('button',{name:'提交运行'}));
  await screen.findByText(/连接中断/);
  expect((screen.getByLabelText('运行输入') as HTMLTextAreaElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole('button',{name:'重试提交'}));
  await waitFor(()=>expect(keys).toHaveLength(2));
  expect(keys[0]).toBe(keys[1]);
  await screen.findByText(/已保存请求/);
});

test.each(['{','{}'])('已接受但回执损坏仍冻结原输入：%s',async body=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(body,{status:202})));
  render(<Start access={{session:{id:'u',name:'成员',admin:false,csrf:'csrf'},project:'p',role:'operator'}}/>);
  fireEvent.click(screen.getByRole('button',{name:'提交运行'}));
  await screen.findByRole('button',{name:'重试提交'});
  expect((screen.getByLabelText('编排 JSON') as HTMLTextAreaElement).disabled).toBe(true);
});

test('切换项目后禁止运行原项目版本',async()=>{
  const fetcher=vi.fn(async()=>new Response(JSON.stringify({project_id:'project-a',kind:'agent'})));vi.stubGlobal('fetch',fetcher);
  render(<Start access={{session:{id:'u',name:'成员',admin:false,csrf:'csrf'},project:'project-b',role:'operator'}} version="version-a"/>);
  fireEvent.click(screen.getByRole('button',{name:'提交运行'}));
  await screen.findByText(/版本不属于当前项目/);
  expect(fetcher).toHaveBeenCalledTimes(1);expect((fetcher.mock.calls[0] as unknown as [string,RequestInit])[1].method).toBe('GET');
});

test('执行前检查失败显示诊断但不冻结尚未提交的输入',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({ready:false,checks:[{component:'worker',status:'unavailable'}]}),{status:503})));
  render(<Start access={{session:{id:'u',name:'成员',admin:false,csrf:'csrf'},project:'p',role:'operator'}}/>);
  fireEvent.click(screen.getByRole('button',{name:'执行前检查'}));
  await screen.findByText(/worker/);
  expect((screen.getByLabelText('编排 JSON') as HTMLTextAreaElement).disabled).toBe(false);
  expect(screen.getByRole('button',{name:'提交运行'})).toBeTruthy();
});
