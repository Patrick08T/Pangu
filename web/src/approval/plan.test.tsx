import {render,screen} from '@testing-library/react';
import {describe,it,expect} from 'vitest';
import {ExecutionPlan} from './plan';
import type {Schema} from '../console/components';

describe('执行图审批',()=>{
  it('显示真实依赖差异、版本和任务映射',()=>{
    const plan={base_version:'original',object_id:'flow',revision:2,flow:{nodes:[]},workflow:{nodes:[],input:''},changes:[{node_id:'qa',before:{id:'qa',agent_version:'agent-old',depends_on:['security']},after:{id:'qa',agent_version:'agent-new',depends_on:['build'],task:'独立核验'}}],assignments:[{task_id:'T-QA',node_id:'qa'}]} satisfies Schema['ExecutionPlanPreview'];
    render(<ExecutionPlan plan={plan}/>);
    expect(screen.getByText('security')).toBeTruthy();
    expect(screen.getByText('build')).toBeTruthy();
    expect(screen.getByText('agent-new')).toBeTruthy();
    expect(screen.getByText('T-QA')).toBeTruthy();
    expect(screen.getByText(/发布新版本/)).toBeTruthy();
  });
});
