import {cleanup,render,screen} from '@testing-library/react';
import {ReportQuality} from './quality';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('历史竞品空报告显示未达到验收，不能把执行成功当作调研完成',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify([{id:'r',title:'空报告',data:{mode:'competition',sources:[{id:'input',read:true}],claims:[{kind:'fact',evidence:[{source_id:'input'}]}]}}]))));
  render(<ReportQuality run="run"/>);
  expect((await screen.findByRole('alert')).textContent).toContain('未达到调研验收');
});

test('有来源仍不声称语义正确，展示人工核验边界',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify([{id:'r',title:'调研',data:{mode:'competition',sources:[{id:'s1',read:true}],claims:[{kind:'fact',evidence:[{source_id:'s1'}]}]}}]))));
  render(<ReportQuality run="run"/>);
  expect((await screen.findByText(/实际读取 1 份资料/)).textContent).toContain('仍需核对');
});
