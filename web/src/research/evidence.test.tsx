import {cleanup,render,screen} from '@testing-library/react';
import {Reports} from './evidence';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('未知结论和未核验证据不会显示为已证实',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify([{id:'report',title:'规划',subject_hash:'hash',reviews:[],data:{mode:'planning',tasks:[],sources:[],claims:[{id:'c1',statement:'效果尚未知',kind:'unknown',scope:'当前团队',uncertainty:'尚未试点',conflicts:['样本口径冲突'],evidence:[]}]}}]))));
  render(<Reports run="r" owner="u" access={{project:'p',role:'viewer',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByText(/效果尚未知/)).toBeTruthy();
  expect(screen.getByText('尚未人工核验')).toBeTruthy();
  expect(screen.getByText('冲突：样本口径冲突')).toBeTruthy();
  expect(screen.queryByRole('button',{name:'保存证据核验'})).toBeNull();
});
