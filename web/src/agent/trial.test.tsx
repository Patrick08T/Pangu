import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {AgentTrial} from './trial';
import type {Access,Schema} from '../console/components';
const item={id:'a',kind:'agent',name:'检查助手',revision:4,archived:false,spec:{agent:{id:'agent',executor:'codex',handoff:{objective:'完成指定任务',inputs:'本次输入',deliverables:'检查报告',acceptance:'说明证据',failure_handling:'报告缺口'},instruction:'检查',read_only:true}}} as Schema['CatalogObject'];
const access={project:'p',role:'manager',session:{id:'u',csrf:'csrf'}} as Access;
const request='10000000-0000-4000-8000-000000000001';
const ready={ready:true,checks:[{component:'worker',status:'ready'}]};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('环境诊断不提交任务，真实试运行固定修订且断网重试复用请求键',async()=>{
 let failed=false;
 const fetch=vi.fn(async(url:string,options?:RequestInit)=>{
  if(url.endsWith('/draft/preflight'))return new Response(JSON.stringify(ready));
  if(options?.method==='POST'){if(!failed){failed=true;throw Error('断网');}return new Response(JSON.stringify({request_id:request,status:'pending'}));}
  return new Response(JSON.stringify(url.includes('/requests/')?{request_id:request,status:'pending'}:{items:[]}));
 });vi.stubGlobal('fetch',fetch);
 render(<AgentTrial item={item} access={access} disabled={false} onUnlock={vi.fn()} onSuccess={vi.fn()} onCheck={vi.fn()} onLock={vi.fn()}/>);
 fireEvent.change(screen.getByLabelText('任务目标'),{target:{value:'检查项目'}});
 fireEvent.click(screen.getByText('检查环境'));await screen.findByText('环境检查完成');
 expect(fetch.mock.calls.filter(([url,options])=>url.endsWith('/trials')&&options?.method==='POST')).toHaveLength(0);
 fireEvent.click(screen.getByText('试运行草稿'));await screen.findByText('重试同一试运行');
 expect((screen.getByLabelText('任务目标') as HTMLTextAreaElement).disabled).toBe(true);
 fireEvent.click(screen.getByText('重试同一试运行'));await screen.findByText('试运行请求已接受');
 const calls=fetch.mock.calls.filter(([url,options])=>url.endsWith('/trials')&&options?.method==='POST');
 expect(calls).toHaveLength(2);expect(JSON.parse(calls[0][1]!.body as string)).toEqual({project_id:'p',revision:4,input:'检查项目'});
 const key=(calls[0][1]!.headers as Headers).get('Idempotency-Key');expect(key).toMatch(/^\d{13}\.[a-f0-9-]{36}$/);
 expect((calls[1][1]!.headers as Headers).get('Idempotency-Key')).toBe(key);
 await waitFor(()=>expect(screen.getByText('试运行结果 · 草稿 r4')).toBeTruthy());
});
