import {act,cleanup,fireEvent,render,screen} from '@testing-library/react';
import {RunDetail} from './detail';
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.useRealTimers();});

test('执行成功但业务验收部分完成时明确显示未通过',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    if(url.includes('/usage?'))return new Response(JSON.stringify({used:{},calls:[]}));
    if(url.startsWith('/api/approvals?'))return new Response(JSON.stringify({items:[]}));
    if(url==='/api/runs/r')return new Response(JSON.stringify({id:'r',project_id:'p',status:'succeeded',stop_confirmed:true,nodes:[],outcome:{state:'needs_attention',nodes:[{node_id:'acceptance',state:'needs_attention',summary:'缺少批准消费证据'}]}}));
    if(url.endsWith('/artifacts'))return new Response(JSON.stringify({files:[]}));
    if(url.endsWith('/debug'))return new Response(JSON.stringify({paused:false,active:0}));
    return new Response(JSON.stringify([]));
  }));
  render(<RunDetail id="r" access={{project:'p',role:'viewer',session:{id:'u',name:'用户',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByText('业务验收未通过')).toBeTruthy();
  expect(screen.getByText('缺少批准消费证据')).toBeTruthy();
  expect(screen.queryByText('业务验收通过')).toBeNull();
});

test('模型等待显示重试时间和轮次，并允许取消',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    if(url.includes('/usage?'))return new Response(JSON.stringify({used:{},calls:[]}));
    if(url.startsWith('/api/approvals?'))return new Response(JSON.stringify({items:[]}));
    if(url==='/api/runs/r')return new Response(JSON.stringify({id:'r',project_id:'p',created_by:'u',status:'revising_plan',created_at:'2026-10-29',stop_confirmed:false,nodes:[{id:'n',node_id:'execution_plan',status:'failed',model_retry:3,review_round:2,next_retry_at:'2026-10-29T08:00:00Z'}]}));
    if(url.endsWith('/artifacts'))return new Response(JSON.stringify({files:[]}));
    if(url.endsWith('/debug'))return new Response(JSON.stringify({paused:false,active:0}));
    return new Response(JSON.stringify([]));
  }));
  render(<RunDetail id="r" access={{project:'p',role:'operator',session:{id:'u',name:'用户',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByText(/execution_plan：等待第 4 次模型重试/)).toBeTruthy();
  expect(screen.getByText(/第 2 轮计划修订/)).toBeTruthy();
  expect(screen.getByText(/下次重试不早于/)).toBeTruthy();
  expect(screen.getByRole('button',{name:'请求取消'})).toBeTruthy();
});
test('待审批详情说明暂停与恢复且保留取消操作',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    if(url.includes('/usage?'))return new Response(JSON.stringify({used:{},calls:[]}));
    if(url.startsWith('/api/approvals?'))return new Response(JSON.stringify({items:[]}));
    if(url==='/api/runs/r')return new Response(JSON.stringify({id:'r',project_id:'p',created_by:'u',status:'waiting_approval',created_at:'2026-10-29',stop_confirmed:false,nodes:[]}));
    if(url.endsWith('/artifacts'))return new Response(JSON.stringify({files:[]}));
    if(url.endsWith('/debug'))return new Response(JSON.stringify({paused:false,active:0}));
    return new Response(JSON.stringify([]));
  }));
  render(<RunDetail id="r" access={{project:'p',role:'operator',session:{id:'u',name:'用户',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByText('待审批')).toBeTruthy();
  expect(screen.getByText(/工作流已暂停，等待人工审批/)).toBeTruthy();
  expect(screen.queryByText(/任务正在推进/)).toBeNull();
  expect(screen.getByRole('button',{name:'请求取消'})).toBeTruthy();
});
test('任务详情优先显示结果，执行日志仅在展开后加载',async()=>{
  const requests:string[]=[];
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    requests.push(url);
    if(url.startsWith('/api/approvals?'))return new Response(JSON.stringify({items:[]}));
    if(url.includes('/usage?'))return new Response(JSON.stringify({used:{},budget:{},calls:[]}));
    if(url.includes('/trace?'))return new Response(JSON.stringify({items:[]}));
    if(url.endsWith('/activity'))return new Response(JSON.stringify([]));
    if(url.endsWith('/debug'))return new Response(JSON.stringify({paused:false,step_requested:false,step_node:'',revision:0,active:0}));
    const data=url==='/api/runs/r'?{id:'r',project_id:'p',created_by:'u',status:'succeeded',created_at:'2026-10-25',stop_confirmed:true,logs_expired:false,nodes:[],definition:{project_id:'p',input:'目标',nodes:[]}}:url.endsWith('/artifacts')?{files:[]}:url.includes('/events')||url.includes('/attempts')?{items:[]}:[];
    return new Response(JSON.stringify(data));
  }));
  render(<RunDetail id="r" access={{project:'p',role:'viewer',session:{id:'u',name:'用户',admin:false,csrf:'csrf'}}}/>);
  await screen.findByRole('heading',{name:'任务结果'});
  expect(screen.getByRole('heading',{name:'Agent 执行状态'})).toBeTruthy();
  expect(requests.some(url=>url.includes('/events')||url.includes('/attempts'))).toBe(false);
  fireEvent.click(screen.getByText('执行过程与日志'));
  await screen.findByRole('heading',{name:'节点尝试与工具证据'});
  expect(requests.some(url=>url.includes('/attempts'))).toBe(true);
});

test('任务轮询后不会重复调试控制或 Agent 图',async()=>{
  vi.useFakeTimers();
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    if(url.endsWith('/debug'))return new Response(JSON.stringify({paused:false,active:0}));
    if(url.endsWith('/artifacts'))return new Response(JSON.stringify({files:[]}));
    if(url==='/api/runs/r')return new Response(JSON.stringify({id:'r',status:'succeeded',nodes:[],stop_confirmed:true,definition:{nodes:[]}}));
    return new Response(JSON.stringify([]));
  }));
  await act(async()=>{render(<RunDetail id="r" access={{project:'p',role:'viewer',session:{id:'u',name:'用户',admin:false,csrf:'csrf'}}}/>);});
  await act(async()=>{await vi.advanceTimersByTimeAsync(10000);});
  expect(screen.getAllByRole('region',{name:'运行调试控制'})).toHaveLength(1);
  expect(screen.getAllByRole('region',{name:'Agent 执行状态'})).toHaveLength(1);
});
