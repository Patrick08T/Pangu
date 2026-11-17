import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Entries} from './entries';
const summary={id:'version-1',entry_id:'entry-1',revision:1,validity:'active',delivery:'shadow',delivery_revision:1,expires_at:'2027-01-01T00:00:00Z',created_at:'2026-11-01T00:00:00Z'};
const permissions={revoke:true,delete:true};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('摘要列表不泄漏额外正文且详情只能显式请求，分页使用服务游标',async()=>{
  const urls:string[]=[];
  vi.stubGlobal('fetch',async(url:string)=>{urls.push(url);return new Response(JSON.stringify(url.includes('cursor=')?{items:[]}:{items:[{...summary,content:{practice:'SECRET-BODY'}}],next_cursor:'cursor&2'}));});
  render(<Entries project="project" csrf="csrf" permissions={permissions}/>);
  await screen.findByText('entry-1');
  expect(document.body.textContent).not.toContain('SECRET-BODY');
  expect(urls).toHaveLength(1);
  fireEvent.click(screen.getByRole('button',{name:'下一页'}));
  await waitFor(()=>expect(urls.some(url=>url.includes('cursor=cursor%262'))).toBe(true));
});

test('详情撤销需要理由与确认，403保留输入且不声称成功',async()=>{
  const detail={...summary,project_id:'project',body_hash:'body-1',content:{kind:'fix',observation:'观察',practice:'做法',conditions:'限定条件',counterexamples:'反例',applicability:{roles:['role']}},sources:[{id:'report',kind:'report',run_id:'run',node_id:'node',project_id:'project',hash:'h'}]};
  const methods:string[]=[];
  vi.stubGlobal('fetch',async(url:string,init:RequestInit)=>{methods.push(init.method??'GET');if(init.method==='POST')return new Response(JSON.stringify({error:'授权已撤销'}),{status:403});return new Response(JSON.stringify(url.includes('/usages')?{items:[]}:url.includes('/version-1')?detail:{items:[summary]}));});
  render(<Entries project="project" csrf="csrf" permissions={permissions}/>);
  fireEvent.click(await screen.findByRole('button',{name:'查看经验 entry-1 修订 1'}));
  await screen.findByText('限定条件');
  const button=screen.getByRole('button',{name:'撤销此经验'});
  expect((button as HTMLButtonElement).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText('变更理由'),{target:{value:'来源授权失效'}});
  fireEvent.click(screen.getByLabelText('确认影响衍生成果与审批依赖'));
  fireEvent.click(button);
  await screen.findByText('授权已撤销');
  expect(methods.filter(method=>method==='POST')).toHaveLength(1);
  expect(screen.queryByText(/经验已撤销/)).toBeNull();
  expect((screen.getByLabelText('变更理由') as HTMLInputElement).value).toBe('来源授权失效');
});

test('列表读取失败不等同没有经验且可重试',async()=>{
  vi.stubGlobal('fetch',async()=>new Response(JSON.stringify({error:'没有项目权限'}),{status:403}));
  render(<Entries project="project" csrf="csrf" permissions={{}}/>);
  await screen.findByRole('alert');
  expect(screen.queryByText('暂无经验记录。')).toBeNull();
  expect(screen.getByRole('button',{name:'重试'})).toBeTruthy();
});
