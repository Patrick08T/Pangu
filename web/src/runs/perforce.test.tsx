import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {PerforceDelivery} from './perforce';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('Perforce 交付展示 shelf 编号并阻止重复创建',async()=>{
 let status='available';
 const fetcher=vi.fn(async(_url:unknown,options?:RequestInit)=>{if(options?.method==='POST')status='shelved';return new Response(JSON.stringify({baseline:12,client:'pangu-test-review',checkpoint:'commit',status,...(status==='shelved'?{changelist:15}:{})}));});vi.stubGlobal('fetch',fetcher);
 render(<PerforceDelivery run="run" allowed stopped csrf="csrf"/>);
 fireEvent.click(await screen.findByRole('button',{name:'生成待审核 changelist'}));
 await screen.findByText(/本轮 changelist：15/);
 await waitFor(()=>expect(screen.queryByRole('button',{name:'生成待审核 changelist'})).toBeNull());
 expect(fetcher.mock.calls.filter(call=>call[1]?.method==='POST')).toHaveLength(1);
});

test('原 changelist 的修改明确提示未包含在本轮 shelf',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>Response.json({baseline:12,client:'daily',checkpoint:'commit',status:'shelved',changelist:15,local_directory:'E:/workspace',existing_changelists:['default','8']})));
 render(<PerforceDelivery run="run" allowed stopped/>);
 await screen.findByText(/未包含在本轮 shelf 中/);
 expect(screen.getByText(/default、8/)).toBeTruthy();
});
