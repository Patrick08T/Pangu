import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Members} from './member';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('禁用成员先说明影响，再请求服务端且只带 CSRF',async()=>{
  const fetcher=vi.fn(async(_url:string,options?:RequestInit)=>options?.method==='POST'?new Response(null,{status:204}):new Response(JSON.stringify([{id:'other',open_id:'ou_other',name:'成员甲',role:'operator',enabled:true,disabled:false}])));
  vi.stubGlobal('fetch',fetcher);
  render(<Members access={{project:'project',role:'manager',session:{id:'admin',name:'管理员',admin:true,csrf:'csrf'}}}/>);
  fireEvent.click(await screen.findByRole('button',{name:'全局禁用'}));
  expect(fetcher.mock.calls.filter(call=>call[1]?.method==='POST')).toHaveLength(0);
  expect(screen.getByText(/未来计划暂停/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button',{name:'确认全局禁用'}));
  await waitFor(()=>expect(fetcher).toHaveBeenCalledWith('/api/users/other/disable',expect.objectContaining({method:'POST'})));
});
