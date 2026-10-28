import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {ScheduleEditor} from './editor';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('编辑计划保留修订号与明确版本，预览标明 UTC',async()=>{
  const captured:unknown[]=[];
  vi.stubGlobal('fetch',vi.fn(async(url:string,options:RequestInit)=>{
    if(url.includes('/preview'))return new Response(JSON.stringify(['2026-10-24T00:00:00Z']));
    if(url.includes('/versions/'))return Response.json({id:'version-a',object_id:'agent-a',project_id:'project',kind:'agent',number:1,spec:{agent:{executor:'eino',instruction:'检查'}}});
    if(url.includes('/catalog?'))return Response.json({items:[]});
    captured.push(JSON.parse(String(options.body)));return new Response(JSON.stringify({id:'plan'}));
  }));
  const plan={id:'plan',name:'每日检查',expression:'0 0 * * *',workflow:{project_id:'project',version_id:'version-a',input:'检查',nodes:[]},revision:7,paused:false,deleted:false,ready:true,created_at:new Date().toISOString(),created_by:'user'};
  const changed=vi.fn();
  render(<ScheduleEditor access={{project:'project',role:'operator',session:{id:'user',name:'成员',admin:false,csrf:'csrf'}}} plan={plan} saved={changed}/>);
  fireEvent.click(screen.getByRole('button',{name:'预览后五次 UTC 触发'}));
  await screen.findByText('2026-10-24T00:00:00.000Z');
  fireEvent.click(screen.getByLabelText('暂停计划'));
  fireEvent.click(screen.getByRole('button',{name:'保存计划'}));
  await waitFor(()=>expect(changed).toHaveBeenCalledOnce());
  expect(captured[0]).toMatchObject({revision:7,paused:true,cron:{workflow:{version_id:'version-a',project_id:'project'}}});
});
test('从画布发布版本创建计划，输入使用表单且无需编排 JSON 或版本 ID',async()=>{
  let submitted:any;
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>{
    if(String(url).includes('/versions/'))return Response.json({id:'version-flow',object_id:'flow',project_id:'project',kind:'workflow',number:2,spec:{flow:{nodes:[],parameters:{type:'object',required:['question'],properties:{question:{type:'string'}}}}}});
    if(String(url).includes('/catalog?'))return Response.json({items:[]});
    submitted=JSON.parse(options.body);return Response.json({id:'plan'});
  }));
  const saved=vi.fn();render(<ScheduleEditor access={{project:'project',role:'operator',session:{id:'user',name:'成员',admin:false,csrf:'csrf'}}} version="version-flow" saved={saved}/>);
  fireEvent.change(screen.getByLabelText(/计划名称/),{target:{value:'每日产品规划'}});
  fireEvent.change(await screen.findByLabelText('研究问题'),{target:{value:'规划需求'}});
  expect(screen.queryByLabelText('固定版本 ID')).toBeNull();expect(screen.queryByLabelText('编排 JSON')).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'保存计划'}));await waitFor(()=>expect(saved).toHaveBeenCalledOnce());
  expect(submitted.workflow).toMatchObject({version_id:'version-flow',input:'{"question":"规划需求"}'});
});
test('原版本归档后仍可暂停计划并原样保留既有快照',async()=>{
  let submitted:any;
  vi.stubGlobal('fetch',vi.fn(async(url,options)=>{
    if(String(url).includes('/versions/'))return Response.json({id:'old',object_id:'flow',project_id:'project',kind:'workflow',archived:true,spec:{flow:{nodes:[]}}});
    if(String(url).includes('/catalog?'))return Response.json({items:[]});
    submitted=JSON.parse(options.body);return Response.json({id:'plan'});
  }));
  const plan={id:'plan',name:'旧计划',expression:'0 0 * * *',workflow:{project_id:'project',version_id:'old',input:'原始输入',nodes:[{id:'one',executor:'eino' as const,instruction:'检查'}]},revision:3,paused:false,deleted:false,ready:true,created_at:'',created_by:'user'};
  const saved=vi.fn();render(<ScheduleEditor access={{project:'project',role:'operator',session:{id:'user',name:'成员',admin:false,csrf:'csrf'}}} plan={plan} saved={saved}/>);
  await screen.findByRole('alert');fireEvent.click(screen.getByLabelText('暂停计划'));fireEvent.click(screen.getByRole('button',{name:'保存计划'}));
  await waitFor(()=>expect(saved).toHaveBeenCalledOnce());expect(submitted).toMatchObject({revision:3,paused:true,cron:{workflow:plan.workflow}});
});
