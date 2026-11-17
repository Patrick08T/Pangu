import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {LoopSettings,loopIssues,markGate} from './loop';
import {issues,type Flow} from './model';

afterEach(cleanup);
const base:Flow={nodes:[{id:'fix',agent_version:'fix-role'},{id:'test',agent_version:'test-role',depends_on:['fix']},{id:'qa',agent_version:'qa-role',depends_on:['test']}],loops:[{id:'bugs',nodes:['fix','test','qa'],gates:[]}]};

test('循环表单能选择质量门，声明只读与必填输出，并保留既有字段',()=>{
  function Editor(){const [flow,setFlow]=useState(base);return <><LoopSettings flow={flow} change={setFlow} disabled={false}/><pre data-testid="value">{JSON.stringify(flow)}</pre></>;}
  render(<Editor/>);
  fireEvent.click(screen.getByLabelText('bugs 质量门：test'));
  fireEvent.click(screen.getByLabelText('bugs 质量门：qa'));
  const value=JSON.parse(screen.getByTestId('value').textContent!);
  expect(value.loops[0].gates).toEqual(['test','qa']);
  expect(value.nodes[1]).toMatchObject({read_only:true,output_schema:{required:['passed','summary'],properties:{passed:{type:'boolean'}}}});
  expect(screen.getByText(/不设轮数上限/)).toBeTruthy();
  expect(loopIssues(value)).toEqual([]);
});

test('移除质量门不隐式授予写权限，重复成员和提前出块会阻止发布',()=>{
  const flow=markGate(base,'bugs','qa',true);
  expect(markGate(flow,'bugs','qa',false).nodes[2].read_only).toBe(true);
  const bad={...flow,loops:[...flow.loops!,{id:'other',nodes:['fix','test'],gates:['test']}]};
  expect(issues(bad).some(p=>p.message.includes('重叠'))).toBe(true);
  expect(issues({...flow,nodes:[...flow.nodes,{id:'consumer',agent_version:'role',depends_on:['fix']}]}).some(p=>p.message.includes('最后质量门'))).toBe(true);
});

test('发布版本只读，不能通过循环配置更改节点',()=>{
  const change=vi.fn();render(<LoopSettings flow={base} change={change} disabled/>);
  expect((screen.getByLabelText('bugs 质量门：qa') as HTMLInputElement).disabled||screen.getByLabelText('bugs 质量门：qa').closest('fieldset')?.disabled).toBe(true);
});
