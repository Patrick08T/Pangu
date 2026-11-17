import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Projects} from './project';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('已有 Perforce 项目可编辑并保存本地目录绑定',async()=>{
 const fetcher=vi.fn(async()=>Response.json({}));vi.stubGlobal('fetch',fetcher);
 const reload=vi.fn();
 render(<Projects access={{project:'p4',role:'read',session:{id:'u',name:'管理员',admin:true,csrf:'csrf'}}} projects={[{id:'p4',name:'P4',vcs:'perforce',worker_id:'worker',source:'',perforce:{port:'localhost:1666',user:'tester',client:'daily',scope:'.'}}]} reload={reload}/>);
 fireEvent.change(screen.getByLabelText('绑定的本地 P4 工作区根目录'),{target:{value:'E:/work'}});
 fireEvent.click(screen.getByRole('button',{name:'检查并保存目录绑定'}));
 await waitFor(()=>expect(reload).toHaveBeenCalledTimes(1));
 const call=fetcher.mock.calls.find((args:unknown[])=>args[0]==='/api/projects/p4/workspace');
 expect(call).toBeTruthy();
});
test('Perforce 登记必须发送本地工作区目录，并可检查目录绑定',async()=>{
 const fetcher=vi.fn(async(_url:unknown,_options?:RequestInit)=>new Response(JSON.stringify({ready:true,checks:[]})));vi.stubGlobal('fetch',fetcher);
 render(<Projects access={{project:'',role:'',session:{id:'u',name:'管理员',admin:true,csrf:'csrf'}}} projects={[]} reload={()=>{}}/>);
 fireEvent.change(screen.getByLabelText('来源类型'),{target:{value:'perforce'}});
 for(const [label,value] of Object.entries({'项目名称':'P4项目','Worker ID':'worker','本地 P4 工作区根目录':'E:/workspace','Perforce 服务器':'localhost:1666','Perforce 用户':'tester','P4 Client':'daily','项目子目录':'Server'}))fireEvent.change(screen.getByLabelText(label),{target:{value}});
 fireEvent.click(screen.getByRole('button',{name:'检查 Perforce 连接'}));
 await screen.findByText(/本地目录、Client 映射和连接检查通过/);
 const checked=JSON.parse(String(fetcher.mock.calls[0][1]?.body));expect(checked).toMatchObject({vcs:'perforce',source:'E:/workspace',perforce:{port:'localhost:1666',user:'tester',client:'daily',scope:'Server'}});expect(checked).not.toHaveProperty('branch');
});
test('验证命令使用参数数组，不接受shell命令字符串',async()=>{
 const fetcher=vi.fn();vi.stubGlobal('fetch',fetcher);
 render(<Projects access={{project:'',role:'',session:{id:'u',name:'管理员',admin:true,csrf:'csrf'}}} projects={[]} reload={()=>{}}/>);
 fireEvent.change(screen.getByLabelText('验证命令 JSON 参数数组'),{target:{value:'"go test ./..."'}});
 fireEvent.submit(screen.getByLabelText('项目名称').closest('form')!);
 await screen.findByText(/必须是字符串数组/);expect(fetcher).not.toHaveBeenCalled();
});
test('离开登记页后的迟到响应不会重载全局项目并销毁新草稿',async()=>{
  let finish!:(response:Response)=>void;
  vi.stubGlobal('fetch',vi.fn(()=>new Promise<Response>(resolve=>{finish=resolve;})));
  const reload=vi.fn(),view=render(<Projects access={{project:'',role:'',session:{id:'u',name:'管理员',admin:true,csrf:'csrf'}}} projects={[]} reload={reload}/>);
  fireEvent.submit(screen.getByLabelText('项目名称').closest('form')!);
  await waitFor(()=>expect(finish).toBeDefined());view.unmount();finish(new Response(JSON.stringify({id:'project'}),{status:201}));
  await new Promise(resolve=>setTimeout(resolve,20));expect(reload).not.toHaveBeenCalled();
});
