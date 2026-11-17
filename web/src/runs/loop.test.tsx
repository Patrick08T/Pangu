import {cleanup,render,screen,within} from '@testing-library/react';
import {LoopProgress,currentNodes} from './loop';
import {WorkflowGraph} from './workflow';
import type {Schema} from '../console/components';

afterEach(cleanup);
const definition:Schema['Workflow']={nodes:[{id:'fix',executor:'codex',instruction:'修复'},{id:'qa',executor:'codex',instruction:'验证',depends_on:['fix']}],loops:[{id:'bugs',nodes:['fix','qa'],gates:['qa']}]};
const nodes=[{id:'new',node_id:'fix',loop_id:'bugs',loop_round:2,status:'running',created_at:'2026-11-03T12:00:00Z'},{id:'old',node_id:'qa',loop_id:'bugs',loop_round:1,status:'succeeded',created_at:'2026-11-03T11:00:00Z',output:'{"passed":false,"summary":"回归失败"}'}] as Schema['NodeRun'][];

test('循环进度保留历史失败，本轮流程图不复用上一轮 QA 状态',()=>{
  expect(currentNodes(nodes).map(n=>n.id)).toEqual(['new']);
  render(<><LoopProgress definition={definition} nodes={nodes}/><WorkflowGraph definition={definition} nodes={nodes}/></>);
  expect(screen.getByText('bugs · 第 2 轮')).toBeTruthy();
  expect(screen.getByText('回归失败')).toBeTruthy();
  const qa=screen.getByRole('button',{name:/qa\s*codex/});
  expect(within(qa).queryByText('成功')).toBeNull();
});

test('按最新尝试选择本轮状态，不依赖返回顺序',()=>{
  const failed:Schema['NodeRun']={...nodes[0],id:'retry-old',status:'failed',created_at:'2026-11-03T11:59:00Z'};
  expect(currentNodes([failed,...nodes]).map(n=>n.id)).toEqual(['new']);
});
