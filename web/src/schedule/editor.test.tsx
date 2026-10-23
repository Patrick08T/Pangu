import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {ScheduleEditor} from './editor';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('编辑计划保留修订号与明确版本，预览标明 UTC',async()=>{
  const captured:unknown[]=[];
  vi.stubGlobal('fetch',vi.fn(async(url:string,options:RequestInit)=>{
    if(url.includes('/preview'))return new Response(JSON.stringify(['2026-10-24T00:00:00Z']));
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
