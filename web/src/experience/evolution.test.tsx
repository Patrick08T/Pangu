import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {EvolutionHistory} from './evolution';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
const item={id:'intent',project_id:'project',policy_revision:4,generation:2,state:'observing',publication:{agent_id:'agent-version',flow_id:'flow-version'},started_at:'2026-11-01T00:00:00Z'};

test('记忆增益实验单独标明真实实验ID且不把意图ID当证据',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({items:[{...item,kind:'memory_benefit',publication:undefined,experiment_id:'real-experiment',benefit_memory_version_id:'memory-version'}]})));
  render(<EvolutionHistory project="project" csrf="csrf" available/>);
  await screen.findByText('记忆增益实验（不修改角色）');
  expect(screen.getByText('real-experiment')).toBeTruthy();
  expect(screen.getByText('memory-version')).toBeTruthy();
  expect(screen.getByText(/实验标识不是机器回执或通过证明/)).toBeTruthy();
  expect(screen.queryByText('角色发布版本')).toBeNull();
});

test('稳定终态已释放占用不能再用旧intent回退',async()=>{
  const stable={...item,state:'stable'};
  vi.stubGlobal('fetch',async(url:string)=>new Response(JSON.stringify(url.endsWith('/intent')?stable:{items:[stable]})));
  render(<EvolutionHistory project="project" csrf="csrf" rollback available/>);
  fireEvent.click(await screen.findByRole('button',{name:'查看进化 intent'}));
  await screen.findByText('当前权限或状态不允许回退。');
  expect(screen.queryByRole('button',{name:'提交受控回退'})).toBeNull();
});

test('冲突后读取新代际保留理由且必须显式核对再提交',async()=>{
  const writes:RequestInit[]=[];let reads=0;
  vi.stubGlobal('fetch',async(url:string,init:RequestInit)=>{
    if(init.method==='POST'){writes.push(init);return writes.length===1?new Response(JSON.stringify({error:'代际冲突'}),{status:409}):new Response(JSON.stringify({...item,generation:3,state:'rolling_back'}),{status:202});}
    if(url.endsWith('/intent'))return new Response(JSON.stringify({...item,generation:++reads===1?2:3}));
    return new Response(JSON.stringify({items:[item]}));
  });
  render(<EvolutionHistory project="project" csrf="csrf" rollback available/>);
  fireEvent.click(await screen.findByRole('button',{name:'查看进化 intent'}));
  fireEvent.change(await screen.findByLabelText('回退理由'),{target:{value:'保留此理由'}});
  fireEvent.click(screen.getByLabelText('仅回退仍由本次晋级控制的未来入口'));
  fireEvent.click(screen.getByRole('button',{name:'提交受控回退'}));
  await screen.findByText('代际冲突');
  fireEvent.click(screen.getByRole('button',{name:'读取最新代际（保留理由）'}));
  fireEvent.click(await screen.findByRole('button',{name:'已核对最新状态，采用 generation 3'}));
  expect((screen.getByLabelText('回退理由') as HTMLInputElement).value).toBe('保留此理由');
  fireEvent.click(screen.getByLabelText('仅回退仍由本次晋级控制的未来入口'));
  fireEvent.click(screen.getByRole('button',{name:'提交受控回退'}));
  await waitFor(()=>expect(writes).toHaveLength(2));
  expect(JSON.parse(String(writes[1].body))).toEqual({generation:3,reason:'保留此理由'});
  expect(screen.queryByText('已回退')).toBeNull();
});

test('执行能力关闭仍显示历史粗状态且不泄漏受保护材料',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({items:[{...item,request:{hidden:'HIDDEN-ANSWER'},decision:{observed:42},experiment_hash:'HIDDEN-HASH'}]})));
  render(<EvolutionHistory project="project" csrf="csrf" rollback={false} available={false}/>);
  await screen.findByText('intent');
  expect(screen.getByText('观察中：尚未成为稳定默认')).toBeTruthy();
  expect(document.body.textContent).not.toContain('HIDDEN-ANSWER');
  expect(document.body.textContent).not.toContain('HIDDEN-HASH');
  expect(screen.queryByText('独立评测通过')).toBeNull();
  expect(screen.queryByRole('button',{name:'提交受控回退'})).toBeNull();
});

test('回退携带固定generation且冲突后不自动重试或宣称成功',async()=>{
  const writes:RequestInit[]=[];
  vi.stubGlobal('fetch',async(url:string,init:RequestInit)=>{if(init.method==='POST'){writes.push(init);return new Response(JSON.stringify({error:'进化代际冲突'}),{status:409});}return new Response(JSON.stringify(url.includes('/intent')&&!url.includes('?')?item:{items:[item]}));});
  render(<EvolutionHistory project="project" csrf="csrf" rollback available/>);
  fireEvent.click(await screen.findByRole('button',{name:'查看进化 intent'}));
  await screen.findByLabelText('回退理由');
  fireEvent.change(screen.getByLabelText('回退理由'),{target:{value:'停止自动采用'}});
  fireEvent.click(screen.getByLabelText('仅回退仍由本次晋级控制的未来入口'));
  fireEvent.click(screen.getByRole('button',{name:'提交受控回退'}));
  await screen.findByText('进化代际冲突');
  expect(JSON.parse(String(writes[0].body))).toEqual({generation:2,reason:'停止自动采用'});
  expect((screen.getByRole('button',{name:'提交受控回退'}) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText('回退理由') as HTMLInputElement).value).toBe('停止自动采用');
  await waitFor(()=>expect(writes).toHaveLength(1));
  expect(screen.queryByText(/回退请求已由服务确认/)).toBeNull();
});
