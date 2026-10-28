import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {FieldSource} from './binding';
import type {Flow,Versions} from './model';
import type {Schema} from '../console/components';

afterEach(cleanup);
const flow:Flow={parameters:{type:'object',properties:{goal:{type:'string'},approved:{type:'boolean'}}},nodes:[{id:'a',agent_version:'v1'},{id:'b',agent_version:'v2',depends_on:['a']},{id:'c',agent_version:'v3'}],editor:{nodes:{a:{name:'产品调研',x:0,y:0}}}};
const versions={v1:{spec:{agent:{output_schema:{type:'object',properties:{'a/b':{type:'string'},score:{type:'number'}}}}}}} as unknown as Versions;
function Form({initial={from:'$input',pointer:'/goal',type:'string'},condition=false}:{initial?:Schema['FieldBinding'];condition?:boolean}){
  const [value,change]=useState(initial);
  return <><FieldSource value={value} change={change} flow={flow} step={flow.nodes[1]} versions={versions} condition={condition}/><output>{JSON.stringify(value)}</output></>;
}
test('通过可读字段名选变量，自动更新类型和转义路径',()=>{
  render(<Form/>);
  expect(screen.getByLabelText('字段路径').closest('details')?.open).toBe(false);
  fireEvent.change(screen.getByLabelText('数据来源'),{target:{value:'a'}});
  fireEvent.change(screen.getByLabelText('选择字段'),{target:{value:'/a~1b'}});
  expect(screen.getByRole('option',{name:'a/b · 文本'})).toBeTruthy();
  expect(JSON.parse(screen.getByRole('status').textContent!)).toEqual({from:'a',pointer:'/a~1b',type:'string'});
  expect(screen.queryByRole('option',{name:'c'})).toBeNull();
});
test('来源改变时重建可选字段，条件只允许文本或布尔值',()=>{
  render(<Form condition initial={{from:'$input',pointer:'/approved',type:'boolean'}}/>);
  fireEvent.change(screen.getByLabelText('数据来源'),{target:{value:'a'}});
  expect(JSON.parse(screen.getByRole('status').textContent!)).toEqual({from:'a',pointer:'/a~1b',type:'string'});
  expect(screen.queryByRole('option',{name:/score/})).toBeNull();
});
test('字段或上游删除后显示失效引用，保留原值等待用户修正',()=>{
  render(<Form initial={{from:'c',pointer:'/removed',type:'number'}}/>);
  expect(screen.getByRole('alert').textContent).toContain('引用已失效');
  expect(JSON.parse(screen.getByRole('status').textContent!)).toEqual({from:'c',pointer:'/removed',type:'number'});
});
test('只读视图仍可展开原始路径与类型核对自定义引用',()=>{
  render(<fieldset disabled><FieldSource value={{from:'a',pointer:'/items/0/text',type:'string'}} change={vi.fn()} flow={flow} step={flow.nodes[1]} versions={{}}/></fieldset>);
  expect(screen.getByText('高级路径').matches(':disabled')).toBe(false);
  fireEvent.click(screen.getByText('高级路径'));
  const input=screen.getByRole('textbox',{name:'字段路径'}) as HTMLInputElement;
  expect(input.value).toBe('/items/0/text');expect(input.matches(':disabled')).toBe(true);
});
