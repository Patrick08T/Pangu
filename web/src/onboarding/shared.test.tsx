import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Onboarding} from './workflow';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('共享工作流和角色分组可选，提交明确的执行项目',async()=>{
  const requests:{url:string;body:unknown}[]=[];
  const version={id:'flow-v1',object_id:'flow',kind:'workflow',number:1,source_revision:1,archived:false,created_at:'2026-10-28',spec:{flow:{nodes:[]}}};
  vi.stubGlobal('fetch',vi.fn(async(url:string,options:RequestInit)=>{
    if(url.includes('/runnables?'))return new Response(JSON.stringify({items:[{name:'性能优化',version},{name:'开发工程师',version:{...version,id:'agent-v1',kind:'agent',spec:{agent:{id:'developer',executor:'codex',instruction:'开发'}}}}]}));
    if(url.includes('/requests/'))return new Response(JSON.stringify({request_id:'00000000-0000-0000-0000-000000000003',status:'pending'}));
    requests.push({url,body:JSON.parse(String(options.body))});
    return new Response(JSON.stringify(url.endsWith('/preflight')?{ready:true,checks:[]}:{request_id:'00000000-0000-0000-0000-000000000003',status:'pending'}));
  }));
  render(<Onboarding access={{project:'project-b',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}}/>);
  const option=await screen.findByRole('option',{name:'性能优化'});
  expect(option.parentElement?.getAttribute('label')).toBe('工作流');
  expect(screen.getByRole('option',{name:'开发工程师'}).parentElement?.getAttribute('label')).toBe('Agent 角色');
  fireEvent.change(screen.getByLabelText('选择助手'),{target:{value:'flow-v1'}});
  fireEvent.click(screen.getByRole('button',{name:'开始执行'}));
  await waitFor(()=>expect(requests).toHaveLength(2));
  expect(requests.map(row=>row.body)).toEqual([{project_id:'project-b',input:''},{project_id:'project-b',input:''}]);
});

test('同名不同配置仍保留选择，并用版本和对象短 ID 区分',async()=>{
  const version={kind:'agent',number:1,source_revision:1,archived:false,created_at:'2026-10-28',spec:{agent:{id:'developer',executor:'codex',instruction:'开发'}}};
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({items:[
    {name:'开发工程师',version:{...version,id:'v1',object_id:'11111111-aaaa'}},
    {name:'开发工程师',version:{...version,id:'v2',object_id:'22222222-bbbb',number:2}},
  ]}))));
  render(<Onboarding access={{project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByRole('option',{name:'开发工程师 · v1 · 11111111'})).toBeTruthy();
  expect(screen.getByRole('option',{name:'开发工程师 · v2 · 22222222'})).toBeTruthy();
});
