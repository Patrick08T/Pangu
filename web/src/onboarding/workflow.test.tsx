import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Onboarding} from './workflow';

const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
const version={id:'v1',object_id:'o1',kind:'agent',project_id:'p',number:1,source_revision:1,archived:false,created_at:'2026-10-25',spec:{agent:{id:'pm',executor:'eino',role:'product_manager',mode:'planning',instruction:'规划'}}};
const ready={ready:true,checks:[{component:'worker',status:'ready'}]};
const accepted={request_id:'00000000-0000-0000-0000-000000000003',status:'pending'};
const response=(body:unknown,status=200)=>new Response(JSON.stringify(body),{status});
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

function service(handle:(url:string,options:RequestInit)=>Promise<Response>) {
  const fetcher=vi.fn(async(url:string,options:RequestInit)=>{
    if(url.includes('/runnables?'))return response({items:[{name:'产品规划助手',version}]});
    if(url.includes('/requests/'))return response(accepted);
    return handle(url,options);
  });
  vi.stubGlobal('fetch',fetcher);return fetcher;
}

test('只有一个助手时自动选择，一次开始先检查再提交，不要求填写背景和版本',async()=>{
  const steps:string[]=[];
  service(async(url,options)=>{steps.push(url);expect(JSON.parse(String(options.body))).toEqual({project_id:'p',input:'{"question":"规划项目"}'});return response(url.endsWith('preflight')?ready:accepted);});
  render(<Onboarding access={access}/>);
  fireEvent.change(await screen.findByLabelText('任务目标'),{target:{value:'规划项目'}});
  expect(screen.queryByLabelText('选择发布版本')).toBeNull();
  expect(screen.queryByRole('button',{name:'检查此任务'})).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'开始执行'}));
  await screen.findByText('已保存请求');
  expect(steps).toEqual(['/api/versions/v1/preflight','/api/versions/v1/run']);
});

test('预检失败给出操作建议，保留输入且不发起运行',async()=>{
  const fetcher=service(async()=>response({ready:false,checks:[{component:'search',status:'unavailable'}]},503));
  render(<Onboarding access={access}/>);
  fireEvent.change(await screen.findByLabelText('任务目标'),{target:{value:'规划项目'}});
  fireEvent.click(screen.getByRole('button',{name:'开始执行'}));
  await screen.findByText(/Codex 联网搜索未就绪/);
  expect((screen.getByLabelText('任务目标') as HTMLTextAreaElement).disabled).toBe(false);
  expect(fetcher.mock.calls.some(([url])=>url.endsWith('/run'))).toBe(false);
});

test('提交结果未知时重试固定请求，不重新检查、不重复生成请求键',async()=>{
  const keys:string[]=[];let checks=0;
  service(async(url,options)=>{
    if(url.endsWith('preflight')){checks++;return response(ready);}
    keys.push(new Headers(options.headers).get('Idempotency-Key')!);
    if(keys.length===1)throw new TypeError('offline');return response(accepted);
  });
  render(<Onboarding access={access}/>);
  fireEvent.change(await screen.findByLabelText('任务目标'),{target:{value:'规划项目'}});
  fireEvent.click(screen.getByRole('button',{name:'开始执行'}));
  await screen.findByRole('button',{name:'重试同一任务'});
  expect((screen.getByLabelText('任务目标') as HTMLTextAreaElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole('button',{name:'重试同一任务'}));
  await screen.findByText('已保存请求');
  expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1]);expect(checks).toBe(1);
});

test('预检等待中重复点击只产生一次提交',async()=>{
  let release:(value:Response)=>void=()=>{};let runs=0;
  service(async url=>url.endsWith('preflight')?new Promise(resolve=>{release=resolve;}):(runs++,response(accepted)));
  render(<Onboarding access={access}/>);
  fireEvent.change(await screen.findByLabelText('任务目标'),{target:{value:'规划项目'}});
  const start=screen.getByRole('button',{name:'开始执行'});fireEvent.click(start);fireEvent.click(start);
  await waitFor(()=>expect((start as HTMLButtonElement).disabled).toBe(true));release(response(ready));
  await screen.findByText('已保存请求');expect(runs).toBe(1);
});

test('没有已发布助手时展示准备入口，不展示无效的运行表单',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>response({items:[]})));
  render(<Onboarding access={access}/>);
  await screen.findByText('还没有已发布的角色或工作流');
  expect(screen.getByRole('link',{name:'准备助手'}).getAttribute('href')).toBe('#/agent');
  expect(screen.queryByRole('button',{name:'开始执行'})).toBeNull();
});
