import {act,cleanup,fireEvent,render,screen,waitFor,within} from '@testing-library/react';
import {RunDebugger} from './debug';
import type {Schema} from '../console/components';

beforeEach(()=>vi.stubGlobal('scrollTo',vi.fn()));
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.useRealTimers();});
const run={id:'r',logs_expired:false,stop_confirmed:true,definition:{input:'用户目标',nodes:[{id:'a',executor:'eino',instruction:'分析'},{id:'b',executor:'codex',instruction:'实现',depends_on:['a']}]},nodes:[{id:'na',node_id:'a',executor:'eino',attempt:2,status:'failed',error:{category:'model',message:'模型请求失败'}},{id:'nb',node_id:'b',executor:'codex',attempt:0,status:'skipped'}]} as Schema['RunDetails'];
function requests(){const fetch=vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/activity')?[]:url.includes('/trace?')?{items:[]}:url.endsWith('/inputs')?[{node_id:'a',state:'ready',parameters:{goal:'调研目标'}},{node_id:'b',state:'skipped',parameters:{}}]:url.includes('/attempts?')?{items:[1,2].map(attempt=>({node_run_id:'na',node_id:'a',attempt,status:'failed'}))}:{node_id:'a',attempt:2,category:'model',message:'本次诊断',evidence:[],truncated:false})));vi.stubGlobal('fetch',fetch);return fetch;}
test('选中运行节点后集中查看输入、错误和所选重试证据，切换不残留上一节点',async()=>{
  const fetch=requests();render(<RunDebugger run={run}/>);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  expect(await screen.findByText(/调研目标/)).toBeTruthy();
  expect(screen.getByText('模型请求失败')).toBeTruthy();
  await screen.findByText('本次诊断');
  fireEvent.change(screen.getByLabelText('执行尝试'),{target:{value:'1'}});
  await waitFor(()=>expect(fetch.mock.calls.some(([url])=>url.endsWith('/nodes/na/attempts/1'))).toBe(true));
  fireEvent.click(screen.getByRole('button',{name:'返回任务'}));
  fireEvent.click(screen.getByRole('button',{name:/b\s*codex/}));
  expect(screen.queryByText('模型请求失败')).toBeNull();expect(screen.queryByText('本次诊断')).toBeNull();
  expect(await screen.findByText('条件未命中或依赖已跳过')).toBeTruthy();
});
test('日志过期不请求输入或证据，也不展示遗留内容',()=>{
  const fetch=requests();render(<RunDebugger run={{...run,logs_expired:true}}/>);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  expect(screen.getByText(/已过保留期/)).toBeTruthy();expect(fetch).not.toHaveBeenCalled();
  expect(screen.queryByText('模型请求失败')).toBeNull();
});
test('尚无执行记录的节点不会生成伪造尝试',async()=>{
  const fetch=requests();render(<RunDebugger run={{...run,nodes:[]}}/>);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  expect(screen.getByText('尚未开始执行')).toBeTruthy();
  await screen.findByText(/调研目标/);
  expect(fetch.mock.calls.some(([url])=>url.includes('/attempts/'))).toBe(false);
});
test('Hatchet 已排队但 Worker 尚未开始时不伪造第 1 次证据',async()=>{
  const fetch=vi.fn(async(url:string)=>url.includes('/attempts/')?new Response(JSON.stringify({error:'节点尝试不存在'}),{status:404}):new Response(JSON.stringify(url.endsWith('/inputs')||url.endsWith('/activity')?[]:{items:[]})));vi.stubGlobal('fetch',fetch);
  render(<RunDebugger run={{...run,stop_confirmed:false,nodes:[{...run.nodes[0],status:'queued',attempt:1,error:undefined}]}}/>);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  await screen.findByText('尚无实际执行尝试。');
  expect(screen.queryByLabelText('执行尝试')).toBeNull();expect(fetch.mock.calls.some(([url])=>url.includes('/attempts/'))).toBe(false);
});

test('停止的任务和旧重试活动不会显示为当前执行',async()=>{
  const event={node_id:'a',node_run_id:'na',attempt:1,kind:'tool',name:'stale_command'};
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/activity')?[event]:url.endsWith('/inputs')?[]:{items:[]}))));
  const current:Schema['RunDetails']={...run,stop_confirmed:false,nodes:[{...run.nodes[0],status:'running',error:undefined}]};
  const view=render(<RunDebugger run={current}/>);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  expect(await screen.findByText('节点执行中，等待下一条模型或工具活动记录。')).toBeTruthy();
  expect(screen.queryByText(/stale_command/)).toBeNull();
  view.rerender(<RunDebugger run={{...current,stop_confirmed:true}}/>);
  expect(screen.getByText('该节点当前没有正在执行的操作，可查看下方历史记录。')).toBeTruthy();
  expect(screen.queryByText(/stale_command/)).toBeNull();
});

test('实时更新失败后明确提示不可确认，不能继续展示旧活动',async()=>{
  vi.useFakeTimers();let failed=false;
  const event={node_id:'a',node_run_id:'na',attempt:2,kind:'tool',name:'running_command'};
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify(failed?{error:'读取失败'}:[event]),{status:failed?502:200})));
  await act(async()=>{render(<RunDebugger run={{...run,stop_confirmed:false,nodes:[{...run.nodes[0],status:'running'}]}}/>);});
  expect(screen.getByText('正在调用工具：running_command')).toBeTruthy();
  failed=true;
  await act(async()=>{await vi.advanceTimersByTimeAsync(2000);});
  expect(screen.getByText('当前活动暂不可用，请勿将旧记录视为仍在执行。')).toBeTruthy();
  expect(screen.queryByText('正在调用工具：running_command')).toBeNull();
});
test('翻页后只选择当前节点实际持久化的尝试',async()=>{
  const fetch=vi.fn(async(url:string)=>new Response(JSON.stringify(url.includes('/trace?')?{items:[]}:url.endsWith('/inputs')?[]:url.includes('/attempts?offset=0')?{items:[],next_offset:100}:url.includes('/attempts?offset=100')?{items:[{node_run_id:'na',node_id:'a',attempt:1}]}:{category:'model',message:'更早的实际诊断',evidence:[]})));vi.stubGlobal('fetch',fetch);
  render(<RunDebugger run={run}/>);fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  fireEvent.click(await screen.findByRole('button',{name:'下一页'}));
  expect(await screen.findByText('更早的实际诊断')).toBeTruthy();
  expect((screen.getByLabelText('执行尝试') as HTMLSelectElement).value).toBe('1');
});

test('主视图显示每个 Agent 状态，点击后展示当前工具及仅该节点的追踪，切换清除旧活动',async()=>{
  const activity={id:8,node_id:'a',node_run_id:'na',attempt:2,kind:'tool',name:'read_project',input:'当前正在读取 README',phase:'start',status:'running',time:'2026-10-28T08:00:00Z'};
  const fetch=vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/activity')?[activity]:url.includes('/trace?')?{items:[{...activity,id:7,phase:'end',status:'succeeded',name:'inspect_file',output:'文件检查完成'}]}:url.includes('/attempts?')?{items:[]}:[])));
  vi.stubGlobal('fetch',fetch);
  render(<RunDebugger run={{...run,stop_confirmed:false,nodes:[{...run.nodes[0],status:'running',error:undefined}]}}/>);
  expect(await screen.findByText('正在调用工具：read_project')).toBeTruthy();
  expect(fetch.mock.calls.some(([url])=>url.includes('/trace?'))).toBe(false);
  fireEvent.click(screen.getByRole('button',{name:/a\s*eino/}));
  const detail=screen.getByRole('complementary',{name:'所选节点详情'});
  expect(within(detail).getByText('当前正在读取 README')).toBeTruthy();
  expect(await within(detail).findByText('文件检查完成')).toBeTruthy();
  expect(fetch.mock.calls.some(([url])=>url.includes('/trace?node=a'))).toBe(true);
  expect(within(detail).queryByLabelText('追踪节点')).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'返回任务'}));
  fireEvent.click(screen.getByRole('button',{name:/b\s*codex/}));
  expect(screen.queryByText('当前正在读取 README')).toBeNull();
  await waitFor(()=>expect(fetch.mock.calls.some(([url])=>url.includes('/trace?node=b'))).toBe(true));
  fireEvent.click(screen.getByRole('button',{name:'返回任务'}));
  expect(screen.queryByRole('complementary',{name:'所选节点详情'})).toBeNull();
});

test('从整页 Agent 详情返回任务后保留所选节点并恢复滚动位置与键盘焦点',async()=>{
  requests();render(<RunDebugger run={run}/>);
  vi.stubGlobal('scrollX',12);vi.stubGlobal('scrollY',640);
  const graph=screen.getByRole('region',{name:'只读流程图'});
  graph.scrollLeft=320;graph.scrollTop=80;
  const card=screen.getByRole('button',{name:/a\s*eino/});
  fireEvent.click(card);
  expect(window.scrollTo).toHaveBeenLastCalledWith(0,0);
  graph.scrollLeft=0;graph.scrollTop=0;
  expect(screen.getByRole('button',{name:'返回任务'})).toBeTruthy();
  await screen.findByText('本次诊断');
  fireEvent.click(screen.getByRole('button',{name:'返回任务'}));
  expect(screen.queryByRole('complementary',{name:'所选节点详情'})).toBeNull();
  expect(card.getAttribute('aria-pressed')).toBe('true');
  expect(document.activeElement).toBe(card);
  expect(graph.scrollLeft).toBe(320);expect(graph.scrollTop).toBe(80);
  expect(window.scrollTo).toHaveBeenLastCalledWith(12,640);
  fireEvent.click(card);
  expect(screen.getByRole('complementary',{name:'所选节点详情'})).toBeTruthy();
  await screen.findByText('本次诊断');
});
