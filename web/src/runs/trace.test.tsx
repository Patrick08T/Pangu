import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {TracePanel,DebugControls} from './trace';
import type {Access,Schema} from '../console/components';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
const run={id:'r',stop_confirmed:false,logs_expired:false,definition:{nodes:[{id:'writer'}]}} as Schema['RunDetails'];
test('按节点筛选、翻页并查看工具参数和父子关联',async()=>{
 const fetch=vi.fn(async(_url:string,_opts?:RequestInit)=>new Response(JSON.stringify({items:[{id:2,kind:'tool',phase:'end',name:'read_project',span_id:'s2',parent_span_id:'s1',input:'document_index=0',output:'资料结果',status:'succeeded',duration_ms:12}],next_before:2})));
 vi.stubGlobal('fetch',fetch);render(<TracePanel run={run}/>);
 expect(await screen.findByText('read_project')).toBeTruthy();
 fireEvent.change(screen.getByLabelText('追踪节点'),{target:{value:'writer'}});
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).includes('node=writer'))).toBe(true));
 fireEvent.click(await screen.findByText('更早记录'));
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).includes('before=2'))).toBe(true));
 expect(screen.getByText(/s1/)).toBeTruthy();
});
test('单步只在暂停且无活动节点时可用，提交当前状态版本',async()=>{
 const fetch=vi.fn(async(_url:string,_opts?:RequestInit)=>new Response(JSON.stringify({paused:true,step_requested:false,step_node:'',active:0,revision:4})));
 vi.stubGlobal('fetch',fetch);
 render(<DebugControls run={run} allowed access={{session:{csrf:'csrf'}} as Access}/>);
 fireEvent.click(await screen.findByRole('button',{name:'单步执行一个节点'}));
 await waitFor(()=>expect(fetch.mock.calls.some(([,opts])=>opts?.body===JSON.stringify({action:'step',revision:4}))).toBe(true));
});
