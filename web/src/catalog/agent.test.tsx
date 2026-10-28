import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {AgentFields} from './agent';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('从常用角色准备助手，用连接选择填入真实模型，无需手写配置',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({items:[{id:'connection-v1',name:'Codex账号',number:1,model:''}]}))));
  function Form(){const[value,setValue]=useState('{"agent":{"id":"agent","executor":"codex","instruction":""}}');return <><AgentFields value={value} change={setValue}/><output>{value}</output></>;}
  render(<Form/>);
  fireEvent.change(screen.getByLabelText('常用角色'),{target:{value:'2'}});
  await screen.findByRole('option',{name:/Codex账号/});
  fireEvent.change(await screen.findByLabelText('共享连接版本'),{target:{value:'connection-v1'}});
  const spec=JSON.parse(screen.getByRole('status').textContent!);
  expect(spec.agent.role).toBe('product_manager');expect(spec.agent.mode).toBe('planning');
  expect(spec.agent.connection_version).toBe('connection-v1');expect(spec.agent.model).toBe('');
  expect(screen.getByLabelText('助手职责')).toBeTruthy();expect(screen.queryByLabelText('配置 JSON')).toBeNull();
});
test('通过表单声明助手输出字段，不修改固定执行器和职责',()=>{
  function Form(){const[value,setValue]=useState('{"agent":{"id":"agent","executor":"codex","instruction":"实现目标"}}');return <><AgentFields value={value} change={setValue}/><output>{value}</output></>;}
  render(<Form/>);
  fireEvent.click(screen.getByLabelText('声明结构化输出'));
  fireEvent.click(screen.getByText('添加字段'));
  fireEvent.change(screen.getByLabelText('字段名 0-1'),{target:{value:'report'}});
  const spec=JSON.parse(screen.getByRole('status').textContent!);
  expect(spec.agent.output_schema).toEqual({type:'object',properties:{report:{type:'string'}}});
  expect(spec.agent.executor).toBe('codex');expect(spec.agent.instruction).toBe('实现目标');
  expect(screen.queryByLabelText('配置 JSON')).toBeNull();
});
