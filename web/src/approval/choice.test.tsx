import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {ReviewForm} from './review';
import type {Schema} from '../console/components';

const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
function gate(mode='single') {
 return {id:'g',project_id:'p',run_id:'r',node_id:'build',owner_id:'u',subject_hash:'a'.repeat(64),status:'waiting',round:1,items:[],review:{title:'选择方案',summary:'明确取舍',resolutions:[],items:[{id:'scope',category:'范围',title:'迭代范围',proposal:'仅文档',rationale:'先验证方向',task_ids:[],evidence_refs:[],selection_mode:mode,options:[{id:'docs',label:'先交付文档',description:'低投入，暂不改变行为'},{id:'code',label:'同步实现功能',description:'可验证产品价值，需要开发和测试'}],recommended_option_ids:['docs'],planned_option_ids:['docs'],recommendation_reason:'方向尚待验证'}]}} as unknown as Schema['ApprovalGate'];
}
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('互斥方案显示推荐和取舍，必须明确选择当前计划才能批准',async()=>{
 const sent:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{sent.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate()} access={access} reload={()=>{}}/>);
 const option=screen.getByRole('radio',{name:'先交付文档'}) as HTMLInputElement;
 expect(option.checked).toBe(false);
 expect(screen.getByText('推荐')).toBeTruthy();
 expect(screen.getByText(/推荐理由：方向尚待验证/)).toBeTruthy();
 const approve=screen.getByRole('button',{name:'全部同意并批准执行'}) as HTMLButtonElement;
 expect(approve.disabled).toBe(true);
 fireEvent.click(option);
 expect(approve.disabled).toBe(false);
 fireEvent.click(approve);
 await waitFor(()=>expect(sent).toEqual([{subject_hash:'a'.repeat(64),decision:'approved',reason:'',items:[{item_id:'scope',decision:'accept',correction:'',selected_option_ids:['docs']}]}]));
});
test('选择替代方案自动纠偏，无需重复输入同一选择',async()=>{
 const sent:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{sent.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate()} access={access} reload={()=>{}}/>);
 fireEvent.click(screen.getByRole('radio',{name:'同步实现功能'}));
 expect((screen.getByRole('button',{name:'全部同意并批准执行'}) as HTMLButtonElement).disabled).toBe(true);
 expect(screen.getByText(/此选择改变了当前计划/)).toBeTruthy();
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 await waitFor(()=>expect(sent).toEqual([{subject_hash:'a'.repeat(64),decision:'changes_requested',reason:'',items:[{item_id:'scope',decision:'revise',correction:'',selected_option_ids:['code']}]}]));
});
test('单选后可清除选择，只提交自定义纠偏意见',async()=>{
 const sent:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{sent.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate()} access={access} reload={()=>{}}/>);
 fireEvent.click(screen.getByRole('radio',{name:'同步实现功能'}));
 fireEvent.click(screen.getByRole('button',{name:'清除方案选择'}));
 expect(screen.getByText(/未决定 1 项/)).toBeTruthy();
 expect((screen.getByRole('radio',{name:'同步实现功能'}) as HTMLInputElement).checked).toBe(false);
 fireEvent.change(screen.getByLabelText('纠偏意见'),{target:{value:'现有方案都不适合，先开展用户访谈'}});
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 await waitFor(()=>expect(sent).toHaveLength(1));
 expect(sent[0]).toMatchObject({decision:'changes_requested',items:[{decision:'revise',correction:'现有方案都不适合，先开展用户访谈'}]});
 expect((sent[0] as Schema['ApprovalDecision']).items![0].selected_option_ids??[]).toEqual([]);
});
test('可组合方案允许多选，清空后恢复未决定，文本仍可纠偏',async()=>{
 const sent:unknown[]=[];
 vi.stubGlobal('fetch',vi.fn(async(_url,init)=>{sent.push(JSON.parse(init.body));return new Response('{}');}));
 render(<ReviewForm gate={gate('multiple')} access={access} reload={()=>{}}/>);
 const docs=screen.getByRole('checkbox',{name:'先交付文档'}),code=screen.getByRole('checkbox',{name:'同步实现功能'});
 fireEvent.click(docs);fireEvent.click(code);
 expect((docs as HTMLInputElement).checked&&(code as HTMLInputElement).checked).toBe(true);
 fireEvent.click(docs);fireEvent.click(code);
 expect(screen.getByText(/未决定 1 项/)).toBeTruthy();
 expect((screen.getByRole('button',{name:'提交纠偏，修订后复审'}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.change(screen.getByLabelText('纠偏意见'),{target:{value:'请补充不修改代码的用户研究方案'}});
 fireEvent.click(screen.getByRole('button',{name:'提交纠偏，修订后复审'}));
 await waitFor(()=>expect(sent).toHaveLength(1));
 expect(sent[0]).toMatchObject({decision:'changes_requested',items:[{decision:'revise',correction:'请补充不修改代码的用户研究方案'}]});
});
