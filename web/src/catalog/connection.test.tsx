import {useState} from 'react';
import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {ConnectionFields,ConnectionPicker} from './connection';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('Codex 账号连接不携带 API 凭据，切换回 API 恢复必填字段',()=>{
  function Form(){const [value,setValue]=useState(JSON.stringify({connection:{base_url:'https://example.com',model:'test',secret_ref:'PANGU_SECRET_TEST',price:{currency:'USD'}}}));return <><ConnectionFields value={value} change={setValue}/><output>{value}</output></>;}
  render(<Form/>);
  fireEvent.change(screen.getByLabelText('连接方式'),{target:{value:'codex'}});
  expect(screen.queryByLabelText('服务端凭据引用')).toBeNull();
  expect(screen.queryByLabelText('模型服务地址')).toBeNull();
  expect(JSON.parse(screen.getByRole('status').textContent!).connection).toEqual({provider:'codex',model:''});
  expect((screen.getByLabelText('模型名称') as HTMLInputElement).required).toBe(false);
  fireEvent.change(screen.getByLabelText('连接方式'),{target:{value:'openai'}});
  expect((screen.getByLabelText('服务端凭据引用') as HTMLInputElement).required).toBe(true);
});
test('翻页查看连接时保留已有固定版本，不清空配置',async()=>{
  const change=vi.fn();vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify({items:url.includes('offset=100')?[]:[{id:'c1',name:'已有账号',number:1,model:''}],next_offset:url.includes('offset=100')?undefined:100}))));
  render(<ConnectionPicker value="c1" change={change}/>);
  await screen.findByRole('option',{name:/已有账号/});fireEvent.click(screen.getByText('更多连接'));
  await waitFor(()=>expect(screen.queryByRole('option',{name:/已有账号/})).toBeNull());
  expect(change).not.toHaveBeenCalled();expect((screen.getByLabelText('共享连接版本') as HTMLSelectElement).value).toBe('c1');
});
