import {useState} from 'react';
import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {CodexModelPicker} from './codex';
import type {Schema} from '../console/components';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('模型设置保存推理强度和 Fast 显式关闭，换模型后提示不支持的组合',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>Response.json({items:[{id:'strong',name:'推理模型',reasoning_levels:['low','high'],default_reasoning:'low',fast:true},{id:'small',name:'轻量模型',reasoning_levels:['low'],default_reasoning:'low'}]})));
  function Form(){const [value,setValue]=useState<Schema['Node']>({id:'a',executor:'codex',instruction:'实现',model:'strong'});return <><CodexModelPicker value={value} change={setValue} disabled={false} editable/><output>{JSON.stringify(value)}</output></>;}
  render(<Form/>);
  await screen.findByRole('option',{name:'推理模型'});
  fireEvent.change(screen.getByLabelText('推理强度'),{target:{value:'high'}});
  fireEvent.change(screen.getByLabelText('Fast 模式'),{target:{value:'false'}});
  expect(JSON.parse(screen.getByRole('status').textContent!)).toMatchObject({reasoning_effort:'high',fast:false});
  fireEvent.change(screen.getByLabelText('Fast 模式'),{target:{value:'true'}});
  fireEvent.change(screen.getByLabelText('Codex 模型'),{target:{value:'small'}});
  expect(screen.getByText(/所选模型不支持当前推理强度/)).toBeTruthy();
  expect(screen.getByText(/所选模型不支持 Fast/)).toBeTruthy();
  expect(JSON.parse(screen.getByRole('status').textContent!)).toMatchObject({reasoning_effort:'high',fast:true});
});
