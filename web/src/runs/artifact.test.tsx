import {cleanup,render,screen} from '@testing-library/react';
import {Artifacts} from './artifact';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('产物出现后默认打开报告正文，并以纯文本显示内容',async()=>{
  const file={id:'report',path:'.pangu/reports/report.md',change:'A',size:32,binary:false,sha256:'hash'};
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/diff')?{file,diff:'## 报告结论\n<script>不执行</script>',truncated:false}:{files:[file]}))));
  const view=render(<Artifacts run="r"/>);
  await screen.findByText(/报告结论/);
  expect(view.container.querySelector('script')).toBeNull();
  expect(screen.getByRole('button',{name:'下载报告'})).toBeTruthy();
});
