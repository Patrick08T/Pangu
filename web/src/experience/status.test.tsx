import {cleanup,render,screen} from '@testing-library/react';
import {CapabilityNotice,DecisionSummary,Lifecycle} from './status';

afterEach(cleanup);

test('真实性通过不被误呈现为默认投放或稳定版本',()=>{
  render(<Lifecycle validity="active" rollout="shadow"/>);
  expect(screen.getByText('已验证真实性')).toBeTruthy();
  expect(screen.getByText('影子验证：不影响业务任务')).toBeTruthy();
  expect(screen.queryByText('稳定版本')).toBeNull();
});

test('观察中与稳定分开且未知状态不冒充成功',()=>{
  const view=render(<Lifecycle validity="candidate" rollout="observing"/>);
  expect(screen.getByText('候选：尚未验证')).toBeTruthy();
  expect(screen.getByText('观察中：尚未成为稳定默认')).toBeTruthy();
  view.rerender(<Lifecycle validity="unrecognized" rollout="future_state"/>);
  expect(screen.getAllByText(/未知状态/)).toHaveLength(2);
});

test('受保护决策只呈现粗粒度结论不泄漏附加字段',()=>{
  const decision={outcome:'rejected',category:'protection_failed',decision_hash:'decision-1',case_id:'SECRET-CASE',expected:'SECRET-ANSWER',observed:123,intervals:[1,2]};
  render(<DecisionSummary decision={decision}/>);
  expect(screen.getByText('未通过')).toBeTruthy();
  expect(screen.getByText('protection_failed')).toBeTruthy();
  expect(screen.getByText('decision-1')).toBeTruthy();
  expect(document.body.textContent).not.toContain('SECRET');
  expect(document.body.textContent).not.toContain('123');
});

test('能力缺失默认关闭而不是模拟通过',()=>{
  render(<CapabilityNotice available={undefined} reason="隔离执行环境尚未接通"/>);
  expect(screen.getByRole('status').textContent).toContain('不可执行');
  expect(screen.getByText(/隔离执行环境尚未接通/)).toBeTruthy();
  expect(screen.getByText(/不代表真实模型收益/)).toBeTruthy();
});
