import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {FlowStudio} from './editor';
import type {Access,Schema} from '../console/components';

vi.mock('./canvas',()=>({FlowCanvas:()=> <div aria-label="流程画布"/>}));
const project='11111111-1111-4111-8111-111111111111';
const version:Schema['CatalogVersion']={id:'22222222-2222-4222-8222-222222222222',object_id:'a',kind:'agent',number:1,source_revision:1,spec:{agent:{id:'agent',executor:'codex',instruction:'检查'}},created_at:'2026-10-26T00:00:00Z',archived:false};
const access:Access={project,role:'manager',session:{id:'user',name:'成员',admin:false,csrf:'csrf'}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('无项目的共享工作流使用角色模板并保留步骤任务，不显示运行入口',async()=>{
  const calls:{url:string;body:any}[]=[];
  const shared={...version,kind:'agent',};
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>{
    const path=String(url),body=options?.body?JSON.parse(options.body):undefined;calls.push({url:path,body});
    if(path.includes('/runnables'))return Response.json({items:[{name:'通用性能角色',version:shared}]});
    if(path.includes('/catalog?'))return Response.json({items:[]});
    if(path.includes('/versions/'))return Response.json(shared);
    if(path.endsWith('/catalog'))return Response.json({id:'shared-flow',kind:'template',name:body.name,spec:body.spec,revision:1,archived:false,created_at:version.created_at});
    if(path.endsWith('/publish'))return Response.json({...shared,id:'flow-published',spec:{flow:{nodes:[]}}});
    return Response.json([]);
  }));
  render(<FlowStudio access={{...access,project:'',role:'',session:{...access.session,admin:true}}}/>);
  fireEvent.change(screen.getByLabelText('工作流名称'),{target:{value:'跨项目性能优化'}});
  fireEvent.click(await screen.findByRole('button',{name:/通用性能角色/}));
  fireEvent.change(screen.getByLabelText('本步骤任务'),{target:{value:'读取基线证据'}});
  fireEvent.click(screen.getByLabelText('本步骤只读'));
  fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
  await screen.findByText('草稿已保存');
  const saved=calls.find(c=>c.url==='/api/catalog')!.body;
  expect(saved.kind).toBe('workflow');expect(saved.project_id).toBeUndefined();
  expect(saved.spec.flow.nodes[0]).toMatchObject({task:'读取基线证据',read_only:true,agent_version:shared.id});
  await waitFor(()=>expect((screen.getByRole('button',{name:'发布'}) as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(screen.getByRole('button',{name:'发布'}));
  await screen.findByText('已发布，任务将使用此固定版本');
  expect(screen.getByRole('link',{name:'运行此版本'}).getAttribute('href')).toBe('#/start/flow-published');
});
test('普通表单添加助手并保存画布，发布和运行不需要配置 JSON',async()=>{
  const calls:{url:string;body:any}[]=[];
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>{
    const path=String(url);const body=options?.body?JSON.parse(options.body):undefined;calls.push({url:path,body});
    if(path.includes('/runnables'))return Response.json({items:[{name:'检查助手',version}]});
    if(path.includes('/catalog?'))return Response.json({items:[]});
    if(path.includes('/versions/'))return Response.json(version);
    if(path.endsWith('/catalog'))return Response.json({id:'flow',kind:'workflow',name:body.name,spec:{flow:{editor:body.spec.flow.editor,nodes:body.spec.flow.nodes,parameters:body.spec.flow.parameters}},revision:1,archived:false,created_at:version.created_at});
    if(path.endsWith('/publish'))return Response.json({...version,id:'published',kind:'workflow'});
    return Response.json([]);
  }));
  render(<FlowStudio access={access}/>);
  fireEvent.change(screen.getByLabelText('工作流名称'),{target:{value:'检查流程'}});
  fireEvent.click(await screen.findByRole('button',{name:/检查助手/}));
  fireEvent.change(screen.getByLabelText('步骤名称'),{target:{value:'检查代码'}});
  expect(screen.queryByLabelText(/配置 JSON/)).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
  await screen.findByText('草稿已保存');
  const saved=calls.find(c=>c.url==='/api/catalog')!.body;
  expect(saved.spec.flow.nodes[0].agent_version).toBe(version.id);
  expect(Object.values(saved.spec.flow.editor.nodes)[0]).toMatchObject({name:'检查代码'});
  await waitFor(()=>expect((screen.getByRole('button',{name:'发布'}) as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(screen.getByRole('button',{name:'发布'}));
  expect((await screen.findByRole('link',{name:'运行此版本'})).getAttribute('href')).toBe('#/start/published');
});
test('只读用户不能添加、修改、保存或发布',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url)=>Response.json(String(url).includes('/versions/')?version:[])));
  const item:Schema['CatalogObject']={id:'flow',kind:'workflow',name:'只读',spec:{flow:{nodes:[{id:'one',agent_version:version.id}]}},revision:1,archived:false,created_at:version.created_at};
  render(<FlowStudio access={{...access,role:'viewer'}} item={item}/>);
  expect((screen.getByLabelText('工作流名称') as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByRole('button',{name:'保存草稿'}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole('button',{name:'发布'}) as HTMLButtonElement).disabled).toBe(true);
});
test('保存请求未完成时仍保护草稿，取消离开不丢输入',async()=>{
  const confirm=vi.fn(()=>false);vi.stubGlobal('confirm',confirm);
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>{if(options?.method==='PUT')return new Promise<Response>(()=>{});return Response.json(String(url).includes('/runnables')?{items:[]}:[]);}));
  const item:Schema['CatalogObject']={id:'flow',kind:'workflow',name:'原名',spec:{flow:{nodes:[]}},revision:1,archived:false,created_at:version.created_at};
  render(<FlowStudio access={access} item={item}/>);fireEvent.change(screen.getByLabelText('工作流名称'),{target:{value:'未保存'}});
  fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));await screen.findByText('正在处理…');
  expect(window.dispatchEvent(new Event('pangu:before-navigate',{cancelable:true}))).toBe(false);
  expect(confirm).toHaveBeenCalledOnce();expect((screen.getByLabelText('工作流名称') as HTMLInputElement).value).toBe('未保存');
});
test('归档工作流可恢复，编辑入口随之开放',async()=>{
  const item:Schema['CatalogObject']={id:'flow',kind:'workflow',name:'已归档流程',spec:{flow:{nodes:[]}},revision:1,archived:true,created_at:version.created_at};
  vi.stubGlobal('fetch',vi.fn(async(url)=>String(url).endsWith('/archive')?new Response(null,{status:204}):Response.json(String(url).endsWith('/catalog/flow')?{...item,revision:2,name:'另一成员的更新',archived:false}:String(url).includes('/runnables')?{items:[]}:[])));
  render(<FlowStudio access={access} item={item}/>);
  fireEvent.click(screen.getByRole('button',{name:'恢复工作流'}));
  await waitFor(()=>expect((screen.getByLabelText('工作流名称') as HTMLInputElement).disabled).toBe(false));
  expect((screen.getByLabelText('工作流名称') as HTMLInputElement).value).toBe('另一成员的更新');
  expect((screen.getByRole('button',{name:'保存草稿'}) as HTMLButtonElement).disabled).toBe(true);
});
test('保存冲突保留本地输入并禁止发布',async()=>{
  const item:Schema['CatalogObject']={id:'flow',kind:'workflow',name:'原名',spec:{flow:{nodes:[]}},revision:1,archived:false,created_at:version.created_at};
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>options?.method==='PUT'?Response.json({error:'conflict'},{status:409}):Response.json(String(url).includes('/runnables')?{items:[]}:[])));
  render(<FlowStudio access={access} item={item}/>);fireEvent.change(screen.getByLabelText('工作流名称'),{target:{value:'我的修改'}});
  fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));await screen.findByRole('alert');
  expect((screen.getByLabelText('工作流名称') as HTMLInputElement).value).toBe('我的修改');
  expect((screen.getByRole('button',{name:'发布'}) as HTMLButtonElement).disabled).toBe(true);
});
