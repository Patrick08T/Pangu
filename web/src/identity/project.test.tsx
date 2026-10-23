import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Projects} from './project';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
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
