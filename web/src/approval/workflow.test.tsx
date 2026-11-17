import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {ApprovalCenter,ApprovalDetail,Approvals} from './workflow';
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
const gate={id:'g',run_id:'r',node_id:'build',project_id:'p',owner_id:'u',round:1,items:[],subject_hash:'a'.repeat(64),subject:{node_input:{parents:{execution_plan:{text:JSON.stringify({ready:true,tasks:[{id:'DEV-1',acceptance:'真实验收条件'}]}),report_ids:['report'],report_sha256:'b'.repeat(64)}}}},status:'waiting',created_at:'2026-01-01T00:00:00Z',review:{title:'本轮计划',summary:'逐项确认',items:[{id:'scope',category:'范围',title:'文档交付',proposal:'仅交付文档',rationale:'先验证',evidence_refs:[],task_ids:[]}],resolutions:[]}};
function mock(status='waiting',stopped=false,review:unknown=gate.review){vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url==='/api/approvals/g'?{approval:{...gate,status,review},run_status:stopped?'failed':'waiting_approval',stopped}:{items:[{id:'g',run_id:'r',node_id:'build',title:'本轮计划',round:1,status,created_at:gate.created_at}]}))));}
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('专用详情逐项审核，无过期时间和默认批准',async()=>{
 mock();render(<ApprovalDetail id="g" access={access}/>);
 expect(await screen.findByText(/不会自动过期/)).toBeTruthy();
 expect(screen.queryByText(/截止/)).toBeNull();
 expect((screen.getByRole('radio',{name:'同意'}) as HTMLInputElement).checked).toBe(false);
});
test('历史过期审批只有查阅入口，不继续计时或允许提交',async()=>{
 mock('expired',true,null);render(<ApprovalDetail id="g" access={access}/>);
 expect(await screen.findByText(/这是旧格式的历史审批/)).toBeTruthy();
 expect(screen.queryByText(/尚未结束/)).toBeNull();
 expect(screen.queryByRole('button',{name:'全部同意并批准执行'})).toBeNull();
});
test('审核详情仍可展开完整依据与冻结摘要',async()=>{
 mock();render(<ApprovalDetail id="g" access={access}/>);
 await screen.findByText('来源材料与冻结内容');
 fireEvent.click(screen.getByText('来源材料与冻结内容'));
 expect(screen.getByRole('heading',{name:'execution_plan · 完整报告'})).toBeTruthy();
 expect(screen.getByText(/检查点：/).textContent).toContain('a'.repeat(64));
 expect(screen.getAllByText(/真实验收条件/).length).toBeGreaterThan(0);
});
test('审核中心及运行摘要提供专用详情链接',async()=>{
 mock();const view=render(<ApprovalCenter access={access}/>);
 expect((await screen.findByRole('link',{name:'打开审核'})).getAttribute('href')).toBe('#/approvals/g');
 view.unmount();render(<Approvals run="r" access={access} stopped={false}/>);
 expect(await screen.findByRole('link',{name:'前往人工审核中心'})).toBeTruthy();
});
test('查看者不能提交结构化审核',async()=>{
 mock();render(<ApprovalDetail id="g" access={{...access,role:'viewer'}}/>);
 expect(await screen.findByText(/只有本项目管理者/)).toBeTruthy();
 expect(screen.queryByRole('radio')).toBeNull();
});

test('已准备修正方案时主说明展示续跑含义，原补证说明留在来源中',async()=>{
 const repair={status:'prepared',summary:'取消多余依赖',reused_nodes:['plan'],execute_nodes:['acceptance'],execution:{changes:[],assignments:[]}};
 vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url==='/api/approvals/g'?{approval:{...gate,purpose:'evidence',repair,review:{...gate.review,summary:'此决定只触发同一只读节点重新核验'}},run_status:'waiting_approval',stopped:false}:{items:[]}))));
 render(<ApprovalDetail id="g" access={access}/>);
 expect(await screen.findByRole('heading',{name:'修正版本与续跑审核'})).toBeTruthy();
 expect(screen.getByText(/本次处置可发布修正版本并复用成果续跑/).className).toBe('intro');
 expect(screen.getByText(/此决定只触发同一只读节点重新核验/).closest('details')?.id).toBe('review-sources');
});
