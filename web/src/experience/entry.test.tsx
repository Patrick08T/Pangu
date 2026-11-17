import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {DefaultEntryRun} from './entry';
const policy={revision:2,revoked:false,entrypoints:['workbench:main','cron:other','webhook:hook','bare','workbench:bad:other']};
const accepted={task_id:'task',request_id:'00000000-0000-4000-8000-000000000001',project_id:'project',operation:'submit',status:'pending',created_at:'2026-11-11T00:00:00Z'};
// JSON wrapper adds 12 bytes: 262132 ASCII, 87377×3+1 CJK, or 65533×4 escaped bytes.
const boundaryInputs=[
  {name:'ASCII',input:'a'.repeat(262132)},
  {name:'中文 UTF-8',input:'界'.repeat(87377)+'a'},
  {name:'引号与反斜杠 JSON 转义',input:'"\\'.repeat(65533)},
];
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals();});

test('入口运行能力未确认时不请求策略或提供模拟提交',()=>{
  const fetcher=vi.fn();vi.stubGlobal('fetch',fetcher);
  render(<DefaultEntryRun project="project" csrf="csrf" available={false} permitted/>);
  expect(screen.getByText(/工作台默认入口运行尚未接通/)).toBeTruthy();
  expect(screen.queryByRole('button',{name:'使用当前已采用版本运行'})).toBeNull();
  expect(fetcher).not.toHaveBeenCalled();
});

test('只列策略授权workbench裸入口且执行权限必须由服务授予',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify(policy)));
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted={false}/>);
  await screen.findByRole('option',{name:'main'});
  expect(screen.queryByRole('option',{name:'other'})).toBeNull();
  expect(screen.queryByRole('option',{name:'bare'})).toBeNull();
  expect(screen.queryByRole('option',{name:'bad:other'})).toBeNull();
  expect((screen.getByRole('button',{name:'使用当前已采用版本运行'}) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/授权候选入口不代表当前已有绑定/)).toBeTruthy();
});

test.each(['连接断开','503','无效回执'])('未知入口提交（%s）冻结输入和幂等键，重试只发input且pending不冒充运行成功',async failure=>{
  const writes:{url:string;init:RequestInit}[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{
    if(init?.method!=='POST')return new Response(JSON.stringify(policy));
    writes.push({url,init});
    if(writes.length===1){
      if(failure==='连接断开')throw new Error('连接断开');
      return failure==='503'?new Response(JSON.stringify({error:'服务暂不可用'}),{status:503}):new Response(JSON.stringify({...accepted,task_id:42}));
    }
    return new Response(JSON.stringify(accepted),{status:202});
  });
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted/>);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:'保留原输入'}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByRole('alert');
  expect(screen.getByText(/输入已冻结/)).toBeTruthy();
  expect((screen.getByLabelText('入口任务输入') as HTMLTextAreaElement).disabled).toBe(true);
  expect((screen.getByLabelText('授权工作台入口') as HTMLSelectElement).disabled).toBe(true);
  fireEvent.click(await screen.findByRole('button',{name:'重试同一入口请求'}));
  await screen.findByRole('link',{name:'查看入口提交回执'});
  expect(writes.map(item=>item.url)).toEqual(['/api/projects/project/entries/main/runs','/api/projects/project/entries/main/runs']);
  expect(JSON.parse(String(writes[0].init.body))).toEqual({input:'保留原输入'});
  expect(writes[1].init.body).toBe(writes[0].init.body);
  const first=new Headers(writes[0].init.headers),second=new Headers(writes[1].init.headers);
  expect(first.get('Idempotency-Key')).toBeTruthy();expect(second.get('Idempotency-Key')).toBe(first.get('Idempotency-Key'));
  expect(second.get('X-CSRF-Token')).toBe('csrf');
  expect(screen.getByText(/请求已登记，尚未确认实际运行/)).toBeTruthy();
});

test('入口提交回执任务标识不合法时不得显示已登记',async()=>{
  vi.stubGlobal('fetch',async(_url:string,init?:RequestInit)=>new Response(JSON.stringify(init?.method==='POST'?{...accepted,task_id:42}:policy)));
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted/>);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:'原输入'}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByText(/入口提交回执范围不符/);
  expect(screen.queryByRole('link',{name:'查看入口提交回执'})).toBeNull();
  expect((screen.getByLabelText('入口任务输入') as HTMLTextAreaElement).value).toBe('原输入');
});

test.each(boundaryInputs)('$name 的 JSON 请求正好256KiB时允许提交',async({input})=>{
  const writes:RequestInit[]=[];
  vi.stubGlobal('fetch',async(_url:string,init?:RequestInit)=>{
    if(init?.method!=='POST')return new Response(JSON.stringify(policy));
    writes.push(init);return new Response(JSON.stringify(accepted),{status:202});
  });
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted/>);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:input}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByRole('link',{name:'查看入口提交回执'});
  expect(writes.length).toBe(1);
  expect(new TextEncoder().encode(String(writes[0].body)).byteLength).toBe(262144);
  expect(JSON.parse(String(writes[0].body))).toEqual({input});
});

test.each(boundaryInputs)('$name 的 JSON 请求超限1字节时零POST且可编辑修正',async({input})=>{
  const oversized=input+'a',writes:RequestInit[]=[],uuid=vi.spyOn(crypto,'randomUUID');
  vi.stubGlobal('fetch',async(_url:string,init?:RequestInit)=>{
    if(init?.method!=='POST')return new Response(JSON.stringify(policy));
    writes.push(init);
    return JSON.parse(String(init.body)).input===oversized?new Response(JSON.stringify({error:'请求体过大'}),{status:413}):new Response(JSON.stringify(accepted),{status:202});
  });
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted/>);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:oversized}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  const alert=await screen.findByRole('alert');
  expect(writes.length).toBe(0);
  expect(uuid).not.toHaveBeenCalled();
  expect(alert.textContent).toMatch(/256\s*KiB/);
  expect(alert.textContent).toMatch(/缩短/);
  expect((screen.getByLabelText('入口任务输入') as HTMLTextAreaElement).disabled).toBe(false);
  expect((screen.getByLabelText('授权工作台入口') as HTMLSelectElement).disabled).toBe(false);
  expect((screen.getByLabelText(/确认使用当前项目已采用版本/) as HTMLInputElement).disabled).toBe(false);
  expect(screen.queryByText(/输入已冻结/)).toBeNull();
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:'缩短后的输入'}});
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByRole('link',{name:'查看入口提交回执'});
  expect(writes.length).toBe(1);expect(uuid).toHaveBeenCalledTimes(1);
  expect(JSON.parse(String(writes[0].body))).toEqual({input:'缩短后的输入'});
});

test('缺少绑定明确失败并保留输入，绝不转为显式版本运行',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{urls.push(url);return init?.method==='POST'?new Response(JSON.stringify({error:'当前入口没有已采用绑定'}),{status:404}):new Response(JSON.stringify(policy));});
  render(<DefaultEntryRun project="project" csrf="csrf" available permitted/>);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:'不可丢失'}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByText('当前入口没有已采用绑定');
  expect((screen.getByLabelText('入口任务输入') as HTMLTextAreaElement).disabled).toBe(true);
  expect((screen.getByLabelText('入口任务输入') as HTMLTextAreaElement).value).toBe('不可丢失');
  expect(screen.queryByRole('link',{name:'查看入口提交回执'})).toBeNull();
  await waitFor(()=>expect(urls.filter(url=>url.endsWith('/runs'))).toEqual(['/api/projects/project/entries/main/runs']));
  expect(urls.some(url=>url.includes('/versions/'))).toBe(false);
});
