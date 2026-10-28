import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Feedback} from './feedback';
import {Quality} from './quality';

const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('反馈绑定读取的产物摘要与修订号，失败运行不能采纳',async()=>{
  const calls:unknown[]=[];
  vi.stubGlobal('fetch',vi.fn(async(_url:string,init?:RequestInit)=>{
    if(init?.method==='PUT'){calls.push(JSON.parse(String(init.body)));return new Response('{}');}
    return new Response(JSON.stringify({subject_hash:'a'.repeat(64),can_accept:false,feedback:null}));
  }));
  render(<Feedback run="r" access={access} owner="u"/>);
  expect((await screen.findByRole('option',{name:'采纳'}) as HTMLOptionElement).disabled).toBe(true);
  fireEvent.change(screen.getByLabelText('评价'),{target:{value:'needs_changes'}});
  fireEvent.change(screen.getByLabelText('理由与验收依据'),{target:{value:'缺少测试'}});
  fireEvent.click(screen.getByRole('button',{name:'保存评价'}));
  await waitFor(()=>expect(calls).toEqual([{revision:0,subject_hash:'a'.repeat(64),decision:'needs_changes',reason:'缺少测试'}]));
});
test('质量展示真实分母，零评定不显示百分之百',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({tasks:1,runs:2,first_reviewed:0,first_accepted:0,reviewed_tasks:0,accepted_tasks:0,needs_changes:0,rejected_tasks:0,unreviewed_tasks:1}))));
  render(<Quality access={access}/>);
  expect(await screen.findByText('暂无已评定样本')).toBeTruthy();
  expect(screen.getByText('业务任务：1 · 运行次数：2 · 未评定任务：1')).toBeTruthy();
});
