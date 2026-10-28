import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {RunDetail} from './detail';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('返工预览后冻结未知提交并沿用请求键，不覆盖旧运行',async()=>{
 const definition={project_id:'p',input:'原任务',nodes:[{id:'build',executor:'codex',instruction:'实现'}]};
 const run={id:'r',project_id:'p',created_by:'u',status:'failed',created_at:'2026-10-25T00:00:00Z',stop_confirmed:true,definition,nodes:[{id:'n',node_id:'build',executor:'codex',status:'succeeded',attempt:1}],events:[],attempts:[]};
 const keys:string[]=[];
 vi.stubGlobal('fetch',vi.fn(async(url:string,options?:RequestInit)=>{
  if(url==='/api/runs/r')return new Response(JSON.stringify(run));
  if(url.endsWith('/fork/preview'))return new Response(JSON.stringify({reused_nodes:['build'],execute_nodes:[],input_changed:false}));
  if(url.endsWith('/fork')){keys.push(new Headers(options?.headers).get('Idempotency-Key')!);throw Error('网络中断');}
  if(url.endsWith('/inputs'))return new Response('[]');
  return new Response('{}',{status:404});
 }));
 render(<RunDetail id="r" access={{project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}}/>);
 fireEvent.click(await screen.findByRole('button',{name:'检查点返工'}));
 fireEvent.click(screen.getByRole('button',{name:'预览返工范围'}));
 await screen.findByText('将复用：build');
 fireEvent.click(screen.getByLabelText('我确认创建独立运行，并承担新增调用消耗'));
 fireEvent.click(screen.getByRole('button',{name:'创建返工运行'}));
 await screen.findByText(/提交结果未知，定义已冻结/);
 expect((screen.getByLabelText('返工定义（JSON）') as HTMLTextAreaElement).disabled).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'重试同一返工提交'}));
 await waitFor(()=>expect(keys).toHaveLength(2));
 expect(keys[0]).toBe(keys[1]);
});
