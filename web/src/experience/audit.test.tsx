import {cleanup,render,screen} from '@testing-library/react';
import {AuditStatus,auditVisible} from './audit';
afterEach(cleanup);

test('仅服务明确证明且未脱敏的依赖才能显示为有效',()=>{
  const view=render(<AuditStatus state={{status:'valid',proven:false,redacted:false}}/>);
  expect(screen.getByText('尚未证明依赖有效')).toBeTruthy();
  view.rerender(<AuditStatus state={{status:'valid',proven:true,redacted:false}}/>);
  expect(screen.getByText('服务已核验依赖有效（仅当前读取时）')).toBeTruthy();
});

test('脱敏优先于valid且权限不足隐藏引用证据',()=>{
  const state={status:'valid',proven:true,redacted:true};
  render(<AuditStatus state={state}/>);
  expect(screen.getByText(/访问受限或内容已脱敏/)).toBeTruthy();
  expect(auditVisible(state)).toBe(false);
  expect(auditVisible({status:'restricted',proven:true,redacted:false})).toBe(false);
  expect(auditVisible({status:'valid',proven:true})).toBe(false);
});

test('脱敏的真实性失效仍明确失效但不暴露证据',()=>{
  render(<AuditStatus state={{status:'invalid',proven:false,redacted:true,invalid_reason:'source_invalidated'}}/>);
  expect(screen.getByText('依赖已失效：不可继续复用')).toBeTruthy();
  expect(screen.getByText(/来源真实性已失效/)).toBeTruthy();
  expect(screen.getByText(/访问受限或内容已脱敏/)).toBeTruthy();
});

test('安全阻断不同于普通暂停，未知原因值不直接回显',()=>{
  const view=render(<AuditStatus state={{status:'invalid',redacted:false,invalid_reason:'SECRET-RAW-REASON'}}/>);
  expect(document.body.textContent).not.toContain('SECRET-RAW-REASON');
  view.rerender(<AuditStatus state={{status:'invalid',redacted:false,invalid_reason:'frozen_safety_blocked'}}/>);
  expect(screen.getByText(/可信机器安全门已阻断/)).toBeTruthy();
});
