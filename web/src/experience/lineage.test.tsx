import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {RunLineage} from './lineage';

const audit={status:'valid',proven:true,redacted:false};
const reference={version_id:'version-1',body_hash:'body-hash',delivery_revision:2};
const artifact={...audit,id:'artifact',node_id:'execute',kind:'output',output_hash:'output-hash',input_hash:'input-hash',execution_input_hash:'execution-hash',generation:3,references:[reference],parents:[{id:'parent/1',hash:'parent-hash'}]};
const page=(items:unknown[]=[])=>({items,total:items.length,has_more:false});
const reply=()=>({actual_run_id:'run',watermark:{...audit,present:true,generation:4,policy_revision:2,hash:'watermark-hash',references:[reference]},usages:page([{...audit,node_id:'execute',iteration:2,approval_generation:3,approval_revision:5,match_reason:'scoped_error_or_keyword_match',injection_hash:'injected-hash',references:[reference],created_at:'2026-11-11T00:00:00Z'}]),artifacts:page([artifact]),chain_complete:false});
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('真实运行影响链展示水位匹配依据与注入哈希但不推断完整或泄漏附加材料',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({...reply(),hidden_expected:'SECRET-ANSWER'})));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText('watermark-hash');
  expect(screen.getByText('项目范围内错误特征或关键词匹配')).toBeTruthy();
  expect(screen.getByText('injected-hash')).toBeTruthy();
  expect(screen.getByText('execution-hash')).toBeTruthy();
  expect(screen.getByText(/当前响应未证明影响链完整/)).toBeTruthy();
  expect(document.body.textContent).not.toContain('SECRET-ANSWER');
});

test('未知匹配原因不回显自由文本',async()=>{
  const value=reply();value.usages.items=[{...value.usages.items[0] as object,...audit,match_reason:'SECRET-RAW-MATCH'}];
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify(value)));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText('匹配原因未登记或无法识别');
  expect(document.body.textContent).not.toContain('SECRET-RAW-MATCH');
});

test('旧任务缺失水位不同于已冻结的空清单',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({...reply(),watermark:{...audit,status:'unproven',proven:false,present:false,generation:0,policy_revision:0,references:[],invalid_reason:'watermark_missing'},usages:page(),artifacts:page()})));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText(/未登记经验水位/);
  expect(screen.queryByText(/已冻结空经验清单/)).toBeNull();
});

test('缺失水位字段的无效回执不能当作合法旧任务',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({...reply(),watermark:{}})));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText('影响链回执不完整，不能视为没有使用经验。');
  expect(screen.queryByText(/未登记经验水位/)).toBeNull();
});

test('已证明的null引用清单按服务空清单契约显示',async()=>{
  const value=reply();
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({...value,watermark:{...value.watermark,references:null},usages:page(),artifacts:page()})));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText(/已冻结空经验清单/);
});

test('受限或脱敏状态即使误带引用和哈希也不展示',async()=>{
  const restricted={...audit,status:'restricted',redacted:true};
  const value=reply();
  value.watermark={...value.watermark,...restricted,hash:'SECRET-WATERMARK',references:[{...reference,version_id:'SECRET-VERSION'}]};
  value.usages=page([{...restricted,node_id:'execute',iteration:1,approval_generation:1,approval_revision:1,match_reason:'SECRET-MATCH',injection_hash:'SECRET-INJECTION',references:[reference]}]);
  value.artifacts=page([{...artifact,...restricted,output_hash:'SECRET-OUTPUT',parents:[{id:'SECRET-PARENT',hash:'SECRET-PARENT-HASH'}]}]);
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify(value)));
  render(<RunLineage project="project" run="run"/>);
  await screen.findAllByText(/访问受限或内容已脱敏/);
  expect(document.body.textContent).not.toContain('SECRET');
});

test('返回其他运行数据时整条影响链隐藏',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({...reply(),actual_run_id:'other'})));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText('返回的运行不匹配，已隐藏影响链。');
  expect(screen.queryByText('watermark-hash')).toBeNull();
});

test('两个游标独立分页且父成果只在明确展开后读取',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{
    urls.push(url);
    if(url.includes('experience-artifacts'))return new Response(JSON.stringify({...artifact,id:'parent/1',parents:[]}));
    return new Response(JSON.stringify({...reply(),usages:{...page(),has_more:true,next_cursor:'usage&next',total:40},artifacts:{...page([artifact]),has_more:true,next_cursor:'artifact&next',total:40}}));
  });
  render(<RunLineage project="project" run="run"/>);
  fireEvent.click(await screen.findByRole('button',{name:'使用记录下一页'}));
  await waitFor(()=>expect(urls.some(url=>url.includes('usage_cursor=usage%26next'))).toBe(true));
  fireEvent.click(await screen.findByRole('button',{name:'成果依赖下一页'}));
  await waitFor(()=>expect(urls.some(url=>url.includes('usage_cursor=usage%26next')&&url.includes('artifact_cursor=artifact%26next'))).toBe(true));
  expect(urls.some(url=>url.includes('experience-artifacts'))).toBe(false);
  fireEvent.click(await screen.findByRole('button',{name:'查看父成果 parent/1'}));
  await waitFor(()=>expect(urls).toContain('/api/projects/project/experience-artifacts/parent%2F1'));
});

test('权限读取失败不降级为无使用且允许重试',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({error:'权限已撤销'}),{status:403}));
  render(<RunLineage project="project" run="run"/>);
  await screen.findByText('权限已撤销');
  expect(screen.queryByText(/已冻结空经验清单/)).toBeNull();
  expect(screen.getByRole('button',{name:'重试'})).toBeTruthy();
});
