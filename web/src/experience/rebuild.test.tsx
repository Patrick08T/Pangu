import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {RunRebuilds} from './rebuild';
const intent={id:'intent',run_id:'run',node_id:'execute',from_generation:2,to_generation:3,reason:'source_revoked',requires_stop:true,created_at:'2026-11-11T00:00:00Z',ready_at:'2026-11-11T00:01:00Z',can_rebuild:true};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('服务明确拒绝代建时保持只读且显示权限原因',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify([{...intent,can_rebuild:false,blocked_reason:'仅原创建人可以重建'}])));
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  await screen.findByText('仅原创建人可以重建');
  expect((screen.getByRole('button',{name:'请求重建并重新审批'}) as HTMLButtonElement).disabled).toBe(true);
});

test('明确确认后只提交空体并将pending后继展示为已登记而非执行成功',async()=>{
  const calls:{url:string;init?:RequestInit}[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{calls.push({url,init});return new Response(JSON.stringify(init?.method==='POST'?{task_id:'task',request_id:'00000000-0000-4000-8000-000000000005',project_id:'project',operation:'rerun',status:'pending',created_at:'2026-11-11T00:00:00Z'}:[intent]),{status:init?.method==='POST'?202:200});});
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  const button=await screen.findByRole('button',{name:'请求重建并重新审批'});
  expect((button as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByLabelText(/我理解这是后继只读重规划/));fireEvent.click(button);
  await screen.findByText(/后继请求已登记，等待服务调度/);
  expect(screen.getByRole('link',{name:'查看后继提交回执'}).getAttribute('href')).toBe('#/requests/00000000-0000-4000-8000-000000000005');
  const post=calls.find(call=>call.init?.method==='POST');
  expect(post?.url).toBe('/api/runs/run/rebuild-intents/intent');
  expect(JSON.parse(String(post?.init?.body))).toEqual({});
  expect(new Headers(post?.init?.headers).get('X-CSRF-Token')).toBe('csrf');
});

test.each([
  {name:'对象请求标识与数值任务标识',request_id:{bad:true},task_id:42},
  {name:'非UUID请求标识',request_id:'not-a-uuid',task_id:'task'},
  {name:'数值任务标识',request_id:'00000000-0000-4000-8000-000000000005',task_id:42},
  {name:'空白任务标识',request_id:'00000000-0000-4000-8000-000000000005',task_id:'   '},
])('无效POST回执不能登记或导航，必须重新核对：$name',async(value)=>{
  vi.stubGlobal('fetch',async(_url:string,init?:RequestInit)=>new Response(JSON.stringify(init?.method==='POST'?{...value,project_id:'project',operation:'rerun',status:'pending',created_at:'2026-11-11T00:00:00Z'}:[intent]),{status:init?.method==='POST'?202:200}));
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  fireEvent.click(await screen.findByLabelText(/我理解这是后继只读重规划/));fireEvent.click(screen.getByRole('button',{name:'请求重建并重新审批'}));
  await screen.findByRole('alert');
  expect(screen.queryByText(/后继请求已登记/)).toBeNull();
  expect(screen.queryByRole('link',{name:'查看后继提交回执'})).toBeNull();
  expect((screen.getByRole('button',{name:'请求重建并重新审批'}) as HTMLButtonElement).disabled).toBe(true);
});

test.each([{bad:true},'not-a-uuid','',null])('GET已存在但无效的后继标识不导航也不允许重复创建：%j',async(successor_request_id)=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify([{...intent,successor_request_id}])));
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  await screen.findByRole('alert');
  expect(screen.queryByText(/服务已有后继提交/)).toBeNull();
  expect(screen.queryByRole('link',{name:'查看后继提交回执'})).toBeNull();
  expect((screen.getByRole('button',{name:'请求重建并重新审批'}) as HTMLButtonElement).disabled).toBe(true);
});

test('GET有效后继标识保持实际回执导航且不重复创建',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify([{...intent,successor_request_id:'00000000-0000-4000-8000-000000000005'}])));
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  expect((await screen.findByRole('link',{name:'查看后继提交回执'})).getAttribute('href')).toBe('#/requests/00000000-0000-4000-8000-000000000005');
  expect(screen.queryByRole('button',{name:'请求重建并重新审批'})).toBeNull();
});

test.each([403,409,503])('重建未获确认不显示成功且必须先刷新核对（HTTP %s）',async(status)=>{
  let posts=0;
  vi.stubGlobal('fetch',async(_url:string,init?:RequestInit)=>{if(init?.method==='POST'){posts++;return new Response(JSON.stringify({error:'旧执行尚未停止'}),{status});}return new Response(JSON.stringify([intent]));});
  render(<RunRebuilds run="run" project="project" csrf="csrf"/>);
  fireEvent.click(await screen.findByLabelText(/我理解这是后继只读重规划/));fireEvent.click(screen.getByRole('button',{name:'请求重建并重新审批'}));
  await screen.findByText('旧执行尚未停止');
  await waitFor(()=>expect(posts).toBe(1));
  expect(screen.queryByRole('link',{name:'查看后继提交回执'})).toBeNull();
  expect((screen.getByRole('button',{name:'请求重建并重新审批'}) as HTMLButtonElement).disabled).toBe(true);
});
