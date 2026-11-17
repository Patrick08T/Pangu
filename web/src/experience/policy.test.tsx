import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {PolicyEditor} from './policy';
import {emptyDraft,serializeDraft} from './policy-fields';

const settings=()=>serializeDraft(emptyDraft());
const permissions={manage_policy:true,manage_sources:true,revoke:true,delete:true,rollback:true};
const features={memory_read:true,claim_verification:true,delivery_promotion:false,evolution:false,run_lineage:false,approval_rebuild:false};
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.restoreAllMocks();});

test('服务未报告费用计价能力时不允许选用费用目标',()=>{
  render(<PolicyEditor policy={{...settings(),revision:1}} project="p" csrf="csrf" permissions={permissions} features={features} supportedMetrics={['acceptance']}/>);
  expect((screen.getByRole('option',{name:'实际调用费用'}) as HTMLOptionElement).disabled).toBe(true);
});

test('未保存编辑在项目切换前请求确认，取消不会丢弃草稿',()=>{
  vi.spyOn(window,'confirm').mockReturnValue(false);
  render(<PolicyEditor policy={{...settings(),revision:1}} project="p" csrf="csrf" permissions={permissions} features={features}/>);
  fireEvent.change(screen.getByLabelText('提炼角色固定版本'),{target:{value:'unsaved'}});
  const event=new Event('pangu:before-navigate',{cancelable:true});
  expect(window.dispatchEvent(event)).toBe(false);
  expect((screen.getByLabelText('提炼角色固定版本') as HTMLInputElement).value).toBe('unsaved');
});

test('只读权限禁用编辑且能力缺失不能启用发布',()=>{
  render(<PolicyEditor policy={{...settings(),revision:1}} project="p" csrf="csrf" permissions={{...permissions,manage_policy:false}} features={features}/>);
  expect((screen.getByRole('button',{name:'保存策略'}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText('自动学习 / 进化总开关') as HTMLInputElement).closest('fieldset')?.disabled).toBe(true);
  expect((screen.getByLabelText('评测通过后自动发布') as HTMLInputElement).disabled).toBe(true);
});

test('双开关独立，真实保存包含CSRF/CAS且不隐式启用学习',async()=>{
  const calls:{url:string;init?:RequestInit}[]=[];
  vi.stubGlobal('fetch',async(url:string,init?:RequestInit)=>{calls.push({url,init});return new Response(JSON.stringify({...settings(),read_enabled:true,revision:3}));});
  render(<PolicyEditor policy={{...settings(),revision:2}} project="p/q" csrf="csrf" permissions={permissions} features={features}/>);
  fireEvent.click(screen.getByLabelText('允许运行读取经验'));
  fireEvent.click(screen.getByRole('button',{name:'保存策略'}));
  await screen.findByText(/策略已保存/);
  expect(calls).toHaveLength(1);expect(calls[0].url).toBe('/api/projects/p%2Fq/experience-policy');
  expect(new Headers(calls[0].init?.headers).get('X-CSRF-Token')).toBe('csrf');
  const body=JSON.parse(String(calls[0].init?.body));
  expect(body.expected_revision).toBe(2);expect(body.policy.enabled).toBe(false);expect(body.policy.read_enabled).toBe(true);
  expect(body.policy.evaluation.non_inferiority).toBeNull();
});

test('CAS冲突保留编辑，读取最新修订也不自动覆盖或再次保存',async()=>{
  const calls:RequestInit[]=[];
  vi.stubGlobal('fetch',async(_url:string,init:RequestInit)=>{calls.push(init);return init.method==='GET'?new Response(JSON.stringify({...settings(),revision:8,roles:['server-role']})):new Response(JSON.stringify({error:'策略修订冲突'}),{status:409});});
  render(<PolicyEditor policy={{...settings(),revision:2}} project="p" csrf="csrf" permissions={permissions} features={features}/>);
  fireEvent.change(screen.getByLabelText('授权角色版本（每行一个）'),{target:{value:'my-role'}});
  fireEvent.click(screen.getByRole('button',{name:'保存策略'}));
  await screen.findByText(/本地编辑已保留/);
  expect((screen.getByLabelText('授权角色版本（每行一个）') as HTMLTextAreaElement).value).toBe('my-role');
  fireEvent.click(screen.getByRole('button',{name:'读取最新修订（保留编辑）'}));
  await screen.findByText(/服务器修订：8/);
  expect((screen.getByLabelText('授权角色版本（每行一个）') as HTMLTextAreaElement).value).toBe('my-role');
  expect(calls.filter(call=>call.method!=='GET')).toHaveLength(1);
  expect((screen.getByRole('button',{name:'保存策略'}) as HTMLButtonElement).disabled).toBe(true);
});

test('不完整保存回执不宣称成功也不清空本地编辑',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({revision:3})));
  render(<PolicyEditor policy={{...settings(),revision:2}} project="p" csrf="csrf" permissions={permissions} features={features}/>);
  fireEvent.change(screen.getByLabelText('提炼角色固定版本'),{target:{value:'keep-me'}});
  fireEvent.click(screen.getByRole('button',{name:'保存策略'}));
  await screen.findByRole('alert');
  expect(screen.queryByText(/策略已保存/)).toBeNull();
  expect((screen.getByLabelText('提炼角色固定版本') as HTMLInputElement).value).toBe('keep-me');
});

test('403请求失败不显示成功且保留输入',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({error:'来源授权已撤销'}),{status:403}));
  render(<PolicyEditor policy={{...settings(),revision:2}} project="p" csrf="csrf" permissions={permissions} features={features}/>);
  fireEvent.change(screen.getByLabelText('提炼角色固定版本'),{target:{value:'draft-version'}});
  fireEvent.click(screen.getByRole('button',{name:'保存策略'}));
  await screen.findByRole('alert');
  await waitFor(()=>expect(screen.queryByText(/策略已保存/)).toBeNull());
  expect((screen.getByLabelText('提炼角色固定版本') as HTMLInputElement).value).toBe('draft-version');
});
