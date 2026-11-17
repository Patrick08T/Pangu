import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {RepairChoice} from './repair';
import type {Schema} from '../console/components';
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:true,csrf:'csrf'}};
const gate={id:'g',project_id:'p',run_id:'r',purpose:'evidence',subject_hash:'gate-hash',status:'waiting'} as Schema['ApprovalGate'];
const repair={hash:'repair-hash',status:'prepared',summary:'取消多余依赖并设置只读',checkpoint_node:'qa_validation',reused_nodes:['plan','qa_validation'],execute_nodes:['security','acceptance'],execution:{base_version:'old',changes:[],assignments:[]}} as unknown as Schema['EvidenceRepair'];
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('显示真实修正方案并要求明确选择，提交绑定方案摘要',async()=>{
 const calls:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{calls.push(JSON.parse(init.body));return new Response('{}');}));
 render(<RepairChoice gate={gate} repair={repair} access={access} reload={()=>{}}/>);
 const approve=screen.getByRole('button',{name:'批准修正版本并续跑'}) as HTMLButtonElement;
 expect(approve.disabled).toBe(true);
 fireEvent.click(screen.getByLabelText('保留历史结论，发布修正版本，复用成果并补跑受影响步骤，再独立验收（推荐）'));
 expect(screen.getByText('将复用：plan、qa_validation')).toBeTruthy();
 fireEvent.click(approve);
 await waitFor(()=>expect(calls).toEqual([{subject_hash:'gate-hash',repair_hash:'repair-hash',decision:'approved',reason:'',items:[]}]));
});
