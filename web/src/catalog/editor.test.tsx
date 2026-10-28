import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {CatalogEditor} from './editor';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('并发保存冲突保留本地草稿，已绑定项目只读且未保存时不能发布',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({error:'草稿修订已变化'}),{status:409})));
  render(<CatalogEditor access={{project:'project-a',role:'operator',session:{id:'user',name:'成员',admin:false,csrf:'csrf'}}} item={{id:'agent',kind:'agent',name:'原名称',project_id:'project-a',spec:{agent:{id:'one',executor:'codex',instruction:'检查'}},revision:2,archived:false,created_at:new Date().toISOString()}} changed={()=>{}}/>);
  expect((screen.getByLabelText('绑定项目（不可迁移）') as HTMLInputElement).readOnly).toBe(true);
  fireEvent.change(screen.getByLabelText('名称'),{target:{value:'本地修改'}});
  expect((screen.getByRole('button',{name:'发布当前草稿'}) as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
  await screen.findByText('草稿修订已变化');
  expect((screen.getByLabelText('名称') as HTMLInputElement).value).toBe('本地修改');
});
