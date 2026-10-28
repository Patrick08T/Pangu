import {act,cleanup,fireEvent,render,screen} from '@testing-library/react';
import {RunDetail} from './detail';
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.useRealTimers();});
test('任务详情优先显示结果，执行日志仅在展开后加载',async()=>{
  const requests:string[]=[];
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    requests.push(url);
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
