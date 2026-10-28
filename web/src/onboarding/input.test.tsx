import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {ConnectionFields} from '../catalog/connection';
import {InputField,RunInput} from './input';
import type {Schema} from '../console/components';

afterEach(cleanup);

function ResearchForm({mode='competition'}:{mode?:string}) {
  const [value,setValue]=useState('{"question":"比较产品","competitors":["Eino","Coze"],"dimensions":["功能"]}');
  return <><form aria-label="调研任务"><RunInput version={{spec:{agent:{id:'pm',executor:'eino',role:'product_manager',mode:mode as 'competition',instruction:'调研'}}}} value={value} change={setValue}/></form><output data-testid="brief">{value}</output></>;
}

test('竞品任务不能无资料启动，用户可明确把候选名称授权为公开搜索词',()=>{
  render(<ResearchForm/>);
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(false);
  expect(screen.getByText(/未读取项目文件/)).toBeTruthy();
  fireEvent.click(screen.getByRole('button',{name:'用竞品名称搜索官方资料'}));
  expect(JSON.parse(screen.getByTestId('brief').textContent!).public_queries).toEqual(['Eino 官方文档','Coze 官方文档']);
  expect((screen.getByLabelText('允许公开发送的搜索词') as HTMLTextAreaElement).value).toContain('Eino 官方文档');
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(true);
});

test('网址或项目文件可替代搜索词，规划保留无联网草案',()=>{
  const view=render(<ResearchForm/>);
  fireEvent.change(screen.getByLabelText('授权公开网址'),{target:{value:'https://example.com/docs'}});
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(true);
  view.unmount();
  render(<ResearchForm mode="planning"/>);
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(true);
  expect(screen.getByText(/仅根据输入生成草案/)).toBeTruthy();
});
test('合法 JSON 的空根不能使连接表单崩溃',()=>{
  render(<ConnectionFields value="null" change={()=>{}}/>);
  expect(screen.getByRole('alert').textContent).toContain('对象');
  expect(screen.getByLabelText('高级连接 JSON')).toBeTruthy();
});

function NumericForm(){
  const [value,setValue]=useState<unknown>({});
  return <><form><InputField shape={{type:'object',properties:{amount:{type:'number'}}}} value={value} change={setValue} label="参数"/></form><output data-testid="payload">{JSON.stringify(value)}</output></>;
}

test('数字精度丢失时拒绝输入，不能将舍入后的值交给服务端',()=>{
  render(<NumericForm/>);
  fireEvent.change(screen.getByLabelText('参数.amount'),{target:{value:'0.10000000000000001'}});
  expect(screen.getByTestId('payload').textContent).toBe('{}');
  expect(screen.getByRole('alert').textContent).toContain('精度');
  fireEvent.change(screen.getByLabelText('参数.amount'),{target:{value:'0.1'}});
  expect(screen.getByTestId('payload').textContent).toBe('{"amount":0.1}');
  expect(screen.queryByRole('alert')).toBeNull();
});

test('省略可选对象不会强制填写它的子字段',()=>{
  render(<form aria-label="参数表单"><InputField label="参数" value={{title:'x'}} change={()=>{}} shape={{type:'object',required:['title'],properties:{title:{type:'string'},options:{type:'object',required:['tag'],properties:{tag:{type:'string'}}}}}}/></form>);
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(true);
});

test('JSON Schema 必填字符串允许显式空串',()=>{
  render(<form><InputField label="标题" shape={{type:'string'}} value="" change={()=>{}} required/></form>);
  expect((screen.getByLabelText('标题') as HTMLTextAreaElement).checkValidity()).toBe(true);
});

function ArrayForm(){
  const [value,setValue]=useState('{"numbers":[1,1]}');
  const version:Schema['CatalogVersion']={id:'v',object_id:'o',kind:'workflow',number:1,source_revision:1,created_at:'2026-10-25T00:00:00Z',archived:false,spec:{flow:{parameters:{type:'object',properties:{numbers:{type:'array',items:{type:'number'}}}},nodes:[]}}};
  return <><form aria-label="数字数组"><RunInput version={version} value={value} change={setValue}/></form><output data-testid="payload">{value}</output></>;
}
test('数字数组保留中间输入并阻止提交，删除相同值的前项不污染后项',()=>{
  render(<ArrayForm/>);
  fireEvent.change(screen.getByLabelText('运行参数.numbers 1'),{target:{value:'-'}});
  expect(screen.getByTestId('payload').textContent).toBe('{"numbers":[1,1]}');
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(false);
  fireEvent.click(screen.getByRole('button',{name:'删除 运行参数.numbers 1'}));
  expect((screen.getByLabelText('运行参数.numbers 1') as HTMLInputElement).value).toBe('1');
  expect((screen.getByRole('form') as HTMLFormElement).checkValidity()).toBe(true);
});
