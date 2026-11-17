import {act,cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {ExperienceConsole,RunExperience} from './console';
const access={project:'project',role:'viewer' as const,session:{id:'u',name:'用户',csrf:'csrf',admin:false}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.restoreAllMocks();});

test('缺失服务权限时策略只读且能力失败不降级为空成功',async()=>{
  vi.stubGlobal('fetch',async(url:string)=>new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{},supported_metrics:[]}:url.includes('policy')?{revision:0}:{items:[]})));
  render(<ExperienceConsole access={access}/>);
  fireEvent.click(await screen.findByRole('button',{name:'策略与授权'}));
  await screen.findByText('服务未授予策略管理权限；仅可查看。');
  expect((screen.getByRole('button',{name:'保存策略'}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText('自动学习 / 进化总开关') as HTMLInputElement).checked).toBe(false);
});

test.each([
  {name:'viewer',role:'viewer',permission:false,csrf:'csrf'},
  {name:'operator',role:'operator',permission:false,csrf:'csrf'},
  {name:'token能力不授予治理',role:'manager',permission:false,csrf:''},
  {name:'缺失permissions',role:'manager',permission:undefined,csrf:'csrf'},
  {name:'非布尔true',role:'manager',permission:'true',csrf:'csrf'},
  {name:'空权限值',role:'manager',permission:null,csrf:'csrf'},
])('$name 不挂载进化历史或发送必然403的查询',async({role,permission,csrf})=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{
    urls.push(url);
    if(url.includes('/evolution/'))return new Response(JSON.stringify({error:'仅治理人员可读'}),{status:403});
    return new Response(JSON.stringify(url.includes('capabilities')?{permissions:permission===undefined?undefined:{manage_policy:permission},features:{evolution:true}}:{items:[]}));
  });
  render(<ExperienceConsole access={{...access,role,session:{...access.session,csrf}}}/>);
  const tab=await screen.findByRole('button',{name:'实验与晋级'});
  await act(async()=>{fireEvent.click(tab);});
  expect(urls.some(url=>url.includes('/evolution/'))).toBe(false);
  expect(screen.getByText(/未授予进化治理权限/)).toBeTruthy();
  expect(screen.queryByRole('heading',{name:'实验与晋级历史'})).toBeNull();
});

test('治理权限已获服务确认时执行能力关闭仍能按需读取历史',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{
    urls.push(url);
    return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{manage_policy:true},features:{evolution:false}}:url.includes('/evolution/')?{items:[{id:'MANAGER-HISTORY',project_id:'project',policy_revision:2,generation:1,state:'stable'}]}:{items:[]}));
  });
  render(<ExperienceConsole access={{...access,role:'manager',session:{...access.session,csrf:''}}}/>);
  const tab=await screen.findByRole('button',{name:'实验与晋级'});
  expect(urls.some(url=>url.includes('/evolution/'))).toBe(false);
  fireEvent.click(tab);
  await screen.findByRole('heading',{name:'MANAGER-HISTORY'});
  expect(urls.filter(url=>url.includes('/evolution/'))).toEqual(['/api/projects/project/evolution/intents?limit=20']);
  expect(screen.getByText(/执行能力关闭不删除历史/)).toBeTruthy();
  expect(screen.queryByText(/未授予进化治理权限/)).toBeNull();
});

test('切换至无治理权限项目后不沿用前一项目历史或查询授权',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{
    urls.push(url);
    return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{manage_policy:url.includes('/project/')},features:{evolution:true}}:url.includes('/evolution/')?{items:[{id:'OLD-GOVERNANCE-HISTORY',project_id:'project',policy_revision:2,generation:1,state:'stable'}]}:{items:[]}));
  });
  const view=render(<ExperienceConsole access={{...access,role:'manager'}}/>);
  fireEvent.click(await screen.findByRole('button',{name:'实验与晋级'}));
  await screen.findByRole('heading',{name:'OLD-GOVERNANCE-HISTORY'});
  view.rerender(<ExperienceConsole access={{...access,project:'other',role:'viewer'}}/>);
  await screen.findByText(/项目：other/);
  await act(async()=>{fireEvent.click(screen.getByRole('button',{name:'实验与晋级'}));});
  expect(urls.filter(url=>url.includes('/evolution/'))).toEqual(['/api/projects/project/evolution/intents?limit=20']);
  expect(screen.getByText(/未授予进化治理权限/)).toBeTruthy();
  expect(screen.queryByRole('heading',{name:'OLD-GOVERNANCE-HISTORY'})).toBeNull();
});

test('默认入口面板按需读取授权入口并传递服务执行权限和CSRF',async()=>{
  const calls:{url:string;init?:RequestInit}[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{calls.push({url,init});return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{submit_entry:true},features:{entry_submission:true}}:url.includes('experience-policy')?{entrypoints:['workbench:main'],revoked:false}:init?.method==='POST'?{request_id:'00000000-0000-4000-8000-000000000001',task_id:'task',project_id:'project',status:'pending'}:{items:[]}));});
  render(<ExperienceConsole access={access}/>);
  const tab=await screen.findByRole('button',{name:'默认入口运行'});
  expect(calls.some(item=>item.url.includes('experience-policy'))).toBe(false);
  fireEvent.click(tab);
  fireEvent.change(await screen.findByLabelText('授权工作台入口'),{target:{value:'main'}});
  fireEvent.change(screen.getByLabelText('入口任务输入'),{target:{value:'运行任务'}});
  fireEvent.click(screen.getByLabelText(/确认使用当前项目已采用版本/));
  fireEvent.click(screen.getByRole('button',{name:'使用当前已采用版本运行'}));
  await screen.findByRole('link',{name:'查看入口提交回执'});
  const submitted=calls.find(item=>item.init?.method==='POST');
  expect(submitted?.url).toBe('/api/projects/project/entries/main/runs');
  expect(new Headers(submitted?.init?.headers).get('X-CSRF-Token')).toBe('csrf');
  expect(JSON.parse(String(submitted?.init?.body))).toEqual({input:'运行任务'});
});

test('项目切换丢弃前一项目展示并按当前项目读取',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{urls.push(url);return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{}}:{items:url.includes('/project/')?[{id:'v',entry_id:'ONLY-OLD-PROJECT',revision:1}]:[]}));});
  const view=render(<ExperienceConsole access={access}/>);
  await screen.findByText('ONLY-OLD-PROJECT');
  view.rerender(<ExperienceConsole access={{...access,project:'other'}}/>);
  await waitFor(()=>expect(urls.some(url=>url.includes('/other/experience-capabilities'))).toBe(true));
  expect(screen.queryByText('ONLY-OLD-PROJECT')).toBeNull();
});

test('运行影响链能力接通后读取真实端点且不再展示缺失占位',async()=>{
  const errors=vi.spyOn(console,'error').mockImplementation(()=>{}),urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{urls.push(url);return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{run_lineage:true,approval_rebuild:false}}:url.includes('/experience?')?{actual_run_id:'run',watermark:{status:'valid',proven:true,redacted:false,present:true,generation:1,policy_revision:2,hash:'hash',references:[]},usages:{items:[],total:0,has_more:false},artifacts:{items:[],total:0,has_more:false},chain_complete:true}:{items:[]}));});
  render(<RunExperience project="project" run="run"/>);
  await screen.findByText(/已冻结空经验清单/);
  expect(urls).toContain('/api/runs/run/experience?limit=20');
  expect(screen.queryByText(/运行级影响链接口尚未接通/)).toBeNull();
  expect(errors).not.toHaveBeenCalled();
});

test('重建能力可用仍以意图服务权限控制按钮',async()=>{
  vi.stubGlobal('fetch',async(url:string)=>new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{approval_rebuild:true}}:url.includes('rebuild-intents')?[{id:'intent',run_id:'run',node_id:'node',from_generation:1,to_generation:2,reason:'source_revoked',requires_stop:true,created_at:'2026-11-11T00:00:00Z',can_rebuild:false,blocked_reason:'当前用户不是原创建人'}]:{items:[]})));
  render(<RunExperience project="project" run="run" csrf="csrf"/>);
  await screen.findByText('当前用户不是原创建人');
  expect((screen.getByRole('button',{name:'请求重建并重新审批'}) as HTMLButtonElement).disabled).toBe(true);
});

test('运行只读真实审批代际并区分系统失效和人的纠偏',async()=>{
  vi.stubGlobal('fetch',async(url:string)=>new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{}}:{items:[{id:'gate',node_id:'execute',round:2,approval_generation:3,status:'invalidated',invalidation_reason:'来源已撤销',invalidated_at:'2026-11-11T00:00:00Z',hidden_expected:'SECRET-EXPECTED'}]})));
  render(<RunExperience project="project" run="run"/>);
  await screen.findByText('审批 generation 3 · 第 2 轮');
  expect(screen.getByText('系统失效：来源已撤销')).toBeTruthy();
  expect(screen.getByText(/系统失效不等于人工纠偏/)).toBeTruthy();
  expect(screen.queryByRole('button',{name:/重新审批/})).toBeNull();
  expect(document.body.textContent).not.toContain('SECRET-EXPECTED');
});

test('运行影响链未接通时不请求虚构接口或提供重审批动作',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{urls.push(url);return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{run_lineage:false,approval_rebuild:false}}:{items:[]}));});
  render(<RunExperience project="project" run="run"/>);
  await screen.findByText(/运行级影响链接口尚未接通/);
  expect(screen.queryByRole('button',{name:/重新审批/})).toBeNull();
  await waitFor(()=>expect(urls).toEqual(['/api/projects/project/experience-capabilities','/api/approvals?project_id=project&run_id=run&offset=0&limit=20']));
});
