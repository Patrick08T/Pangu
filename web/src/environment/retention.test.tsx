import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Retention} from './retention';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('清理必须确认，执行后可查询独立历史记录',async()=>{
 const item={run_id:'run',project_id:'project',created_at:'2026-01-01T00:00:00Z',status:'eligible'};
 const fetcher=vi.fn(async(url:string,options?:RequestInit)=>new Response(JSON.stringify(url.includes('/history')?{items:[{id:'1',run_id:'old-run',project_id:'project',worker_id:'worker',status:'purged',checked_at:'2026-01-02T00:00:00Z'}]}:{items:[{...item,status:options?.method==='POST'?'purged':'eligible'}]}),{status:200}));
 vi.stubGlobal('fetch',fetcher);
 render(<Retention access={{project:'',role:'',session:{id:'u',name:'管理员',admin:true,csrf:'csrf'}}}/>);
 await screen.findByText('可清理');
 const button=screen.getByRole('button',{name:'执行一批清理'});
 expect((button as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByRole('checkbox'));fireEvent.click(button);
 await waitFor(()=>expect(fetcher.mock.calls.some(call=>call[1]?.method==='POST')).toBe(true));
 await screen.findByText('old-run');
 expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(false);
 expect(fetcher.mock.calls.some(call=>call[1]?.method==='POST'&&new Headers(call[1]?.headers).get('X-CSRF-Token')==='csrf')).toBe(true);
});
