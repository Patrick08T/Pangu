import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {CreateCatalog} from './create';

afterEach(()=>{cleanup();vi.unstubAllGlobals();window.location.hash='';});
test('切换项目后旧创建回执不会导航离开新草稿',async()=>{
  let finish!:(response:Response)=>void;
  vi.stubGlobal('fetch',vi.fn(()=>new Promise<Response>(resolve=>{finish=resolve;})));
  const view=render(<CreateCatalog access={{project:'a',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}} kind="agent" created={vi.fn()}/>);
  fireEvent.change(screen.getByLabelText('名称'),{target:{value:'旧配置'}});
  fireEvent.submit(screen.getByLabelText('名称').closest('form')!);
  await waitFor(()=>expect(finish).toBeDefined());view.unmount();window.location.hash='/workflow';
  finish(new Response(JSON.stringify({id:'old-result'}),{status:201}));
  await new Promise(resolve=>setTimeout(resolve,20));
  expect(window.location.hash).toBe('#/workflow');
});
