import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Approvals} from './workflow';
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('审批显示确切待执行摘要并提交显式人工决定',async()=>{
  const calls:unknown[]=[];
  vi.stubGlobal('fetch',vi.fn(async(_path:string,init?:RequestInit)=>{
    if(init?.method==='PUT'){calls.push(JSON.parse(String(init.body)));return new Response('{}');}
    return new Response(JSON.stringify([{id:'g',run_id:'r',node_id:'build',project_id:'p',owner_id:'u',subject_hash:'a'.repeat(64),subject:{node_input:{input:'已确认方案'}},status:'waiting',created_at:new Date().toISOString(),expires_at:new Date(Date.now()+60000).toISOString()}]));
  }));
  render(<Approvals access={access} run="r" stopped={false}/>);
  expect(await screen.findByText(/已确认方案/)).toBeTruthy();
  expect((screen.getByLabelText('审核决定') as HTMLSelectElement).value).toBe('');
  fireEvent.change(screen.getByLabelText('审核决定'),{target:{value:'approved'}});
  fireEvent.change(screen.getByLabelText('审核理由'),{target:{value:'范围与验收条件已核对'}});
  fireEvent.click(screen.getByRole('button',{name:'提交审核决定'}));
  await waitFor(()=>expect(calls).toEqual([{subject_hash:'a'.repeat(64),decision:'approved',reason:'范围与验收条件已核对'}]));
});
