import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Routes} from '../console/routes';
import {RunDetail} from '../runs/detail';
const access={project:'p',role:'viewer' as const,session:{id:'u',name:'用户',csrf:'csrf',admin:false}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('项目路由挂载真实经验控制台',async()=>{
  vi.stubGlobal('fetch',async(url:string)=>new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{}}:{items:[]})));
  render(<Routes route="/experience" access={access} projects={[]} reload={()=>{}}/>);
  await screen.findByRole('heading',{name:'经验与进化'});
  await screen.findByRole('button',{name:'策略与授权'});
});
test('任务经验面板只在同项目显式展开后请求能力',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{urls.push(url);return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{}}:url==='/api/runs/run'?{id:'run',project_id:'p',status:'succeeded',stop_confirmed:true,nodes:[]}:url.endsWith('/artifacts')?{files:[]}:url.endsWith('/debug')?{active:0}:url.includes('/usage?')?{used:{},calls:[]}:url.includes('/approvals?')?{items:[]}:[]));});
  render(<RunDetail id="run" access={access}/>);
  const summary=await screen.findByText('经验依据与失效影响');
  expect(urls.some(url=>url.includes('experience-capabilities'))).toBe(false);
  fireEvent.click(summary);
  await screen.findByText(/运行级影响链接口尚未接通/);
  await waitFor(()=>expect(urls.some(url=>url.includes('/projects/p/experience-capabilities'))).toBe(true));
});

test('运行详情把交互会话CSRF传给权限已核验的重建操作',async()=>{
  const posts:RequestInit[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{
    if(init?.method==='POST'){posts.push(init);return new Response(JSON.stringify({task_id:'task',request_id:'00000000-0000-4000-8000-000000000005',project_id:'p',operation:'rerun',status:'pending',created_at:'2026-11-11T00:00:00Z'}),{status:202});}
    return new Response(JSON.stringify(url.includes('capabilities')?{permissions:{},features:{approval_rebuild:true}}:url.endsWith('rebuild-intents')?[{id:'intent',run_id:'run',node_id:'node',from_generation:1,to_generation:2,reason:'source_revoked',requires_stop:true,created_at:'2026-11-11T00:00:00Z',can_rebuild:true}]:url==='/api/runs/run'?{id:'run',project_id:'p',created_by:'u',status:'failed',stop_confirmed:true,logs_expired:true,nodes:[]}:url.endsWith('/artifacts')?{files:[]}:url.endsWith('/debug')?{active:0}:url.includes('/usage?')?{used:{},calls:[]}:url.includes('/approvals?')?{items:[]}:[]));
  });
  render(<RunDetail id="run" access={{...access,role:'editor'}}/>);
  fireEvent.click(await screen.findByText('经验依据与失效影响'));
  fireEvent.click(await screen.findByLabelText(/我理解这是后继只读重规划/));
  fireEvent.click(screen.getByRole('button',{name:'请求重建并重新审批'}));
  await screen.findByRole('link',{name:'查看后继提交回执'});
  expect(new Headers(posts[0].headers).get('X-CSRF-Token')).toBe('csrf');
});
