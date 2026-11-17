import {emptyDraft,readDraft,serializeDraft,validateDraft} from './policy-fields';

test('新策略没有默认启用或默认容差且保存草稿不把空容差当零',()=>{
  const draft=emptyDraft();
  expect(draft.enabled).toBe(false);expect(draft.read_enabled).toBe(false);
  expect(draft.values['evaluation.non_inferiority']).toBe('');
  expect(draft.values['evaluation.minimum_improvement']).toBe('');
  const body=serializeDraft(draft);
  expect(body.evaluation.non_inferiority).toBeNull();
  expect(body.evaluation.resource_tolerance).toBeNull();
});

test('启用必须显式填写所有授权、预算与阈值，不能只打开开关',()=>{
  const draft=emptyDraft();draft.enabled=true;
  expect(validateDraft(draft)).toContain('授权角色版本');
  draft.values.roles='role-v1';
  expect(validateDraft(draft)).toContain('提炼角色固定版本');
  draft.values.extractor_version='extract-v1';draft.values.generator_version='gen-v1';draft.values.environments='sandbox';
  expect(validateDraft(draft)).toContain('学习调用额度');
});

test('已登记零容差与未填写不同，读写双开关互不覆盖',()=>{
  const original=serializeDraft(emptyDraft());
  original.read_enabled=true;original.evaluation.non_inferiority=0;
  const draft=readDraft({...original,revision:2});
  expect(draft.enabled).toBe(false);expect(draft.read_enabled).toBe(true);
  expect(draft.values['evaluation.non_inferiority']).toBe('0');
  expect(draft.values['evaluation.resource_tolerance']).toBe('');
  expect(serializeDraft(draft).read_enabled).toBe(true);
});

test('空白与无穷数字不能隐式变成授权阈值，ID列表去空行不造默认',()=>{
  const draft=emptyDraft();draft.values.roles=' role-a\n\n role-b ';
  draft.values['evaluation.resource_tolerance']='Infinity';
  expect(()=>serializeDraft(draft)).toThrow(/资源增量容差/);
  draft.values['evaluation.resource_tolerance']='0';
  expect(serializeDraft(draft).roles).toEqual(['role-a','role-b']);
  expect(serializeDraft(draft).evaluation.resource_tolerance).toBe(0);
});
