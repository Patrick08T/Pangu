import {cleanup,fireEvent,render,screen,waitFor,within} from '@testing-library/react';
import {ReviewForm} from './review';
import type {Schema} from '../console/components';
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
const gate={id:'g',project_id:'p',run_id:'r',node_id:'build',owner_id:'u',subject_hash:'a'.repeat(64),status:'waiting',round:1,items:[],review:{title:'方向与范围',summary:'逐项核对',items:['direction','scope'].map((id,i)=>({id,category:'产品',title:i?'实施范围':'产品方向',proposal:'仅文档',rationale:'验证需求',task_ids:[],evidence_refs:[]})),resolutions:[]}} as unknown as Schema['ApprovalGate'];
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('未默认同意，所有项目明确通过才允许批准',async()=>{
 const calls:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{calls.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate} access={access} reload={()=>{}}/>);
 const approve=screen.getByRole('button',{name:'全部同意并批准执行'}) as HTMLButtonElement;
 expect(approve.disabled).toBe(true);
 for(const title of ['产品方向','实施范围'])fireEvent.click(within(screen.getByRole('group',{name:title})).getByLabelText('同意'));
 expect(approve.disabled).toBe(false);
 fireEvent.click(approve);
 await waitFor(()=>expect(calls).toEqual([{subject_hash:gate.subject_hash,decision:'approved',reason:'',items:[{item_id:'direction',decision:'accept',correction:''},{item_id:'scope',decision:'accept',correction:''}]}]));
});
test('输入纠偏自动改为需调整，允许先提交局部意见',async()=>{
 const calls:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{calls.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate} access={access} reload={()=>{}}/>);
 const item=within(screen.getByRole('group',{name:'产品方向'}));
 fireEvent.click(item.getByLabelText('同意'));
 fireEvent.change(item.getByLabelText('纠偏意见'),{target:{value:'聚焦个人开发者'}});
 expect((item.getByLabelText('需调整') as HTMLInputElement).checked).toBe(true);
 expect((screen.getByRole('button',{name:'全部同意并批准执行'}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 await waitFor(()=>expect(calls).toEqual([{subject_hash:gate.subject_hash,decision:'changes_requested',reason:'',items:[{item_id:'direction',decision:'revise',correction:'聚焦个人开发者'}]}]));
});
test('未解决的上一轮意见不能批准',()=>{
 render(<ReviewForm gate={{...gate,review:{...gate.review!,resolutions:[{item_id:'scope',status:'unresolved',explanation:'缺少证据'}]}}} access={access} reload={()=>{}}/>);
 for(const title of ['产品方向','实施范围'])fireEvent.click(within(screen.getByRole('group',{name:title})).getByLabelText('同意'));
 expect((screen.getByRole('button',{name:'全部同意并批准执行'}) as HTMLButtonElement).disabled).toBe(true);
});

test('网络结果不明时锁定表单并重试相同决定',async()=>{
 const requests:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{requests.push(JSON.parse(init.body));if(requests.length===1)throw new Error('网络中断');return new Response('{}');}));
 render(<ReviewForm gate={gate} access={access} reload={()=>{}}/>);
 fireEvent.change(within(screen.getByRole('group',{name:'产品方向'})).getByLabelText('纠偏意见'),{target:{value:'收窄范围'}});
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 const retry=await screen.findByRole('button',{name:'重试同一审核决定'});
 expect((screen.getByRole('group',{name:'逐项审核'}) as HTMLFieldSetElement).disabled).toBe(true);
 fireEvent.click(retry);
 await waitFor(()=>expect(requests).toHaveLength(2));
 expect(requests[1]).toEqual(requests[0]);
});

test('未提交纠偏可阻止导航，冲突后要求重新读取',async()=>{
 vi.stubGlobal('confirm',vi.fn(()=>false));
 vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({error:'内容已变化'}),{status:409})));
 const reload=vi.fn();
 render(<ReviewForm gate={gate} access={access} reload={reload}/>);
 fireEvent.change(within(screen.getByRole('group',{name:'产品方向'})).getByLabelText('纠偏意见'),{target:{value:'收窄范围'}});
 const leave=new Event('pangu:before-navigate',{cancelable:true});
 window.dispatchEvent(leave);
 expect(leave.defaultPrevented).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 fireEvent.click(await screen.findByRole('button',{name:'重新读取审核状态'}));
 expect(reload).toHaveBeenCalledOnce();
});
