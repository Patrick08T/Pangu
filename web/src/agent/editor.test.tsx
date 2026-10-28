import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {AgentStudio} from './editor';
import type {Access,Schema} from '../console/components';

const access={project:'p',role:'manager',session:{id:'u',csrf:'csrf'}} as Access;
const item={id:'a',kind:'agent',project_id:'p',name:'产品助手',revision:1,archived:false,created_at:'2026-10-26T00:00:00Z',spec:{agent:{id:'agent',executor:'codex',handoff:{objective:'完成指定任务',inputs:'本次输入',deliverables:'检查报告',acceptance:'说明证据',failure_handling:'报告缺口'},instruction:'原始职责',read_only:true}}} as Schema['CatalogObject'];
afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals();});

test('缺少交接契约的草稿可保存，但发布与试运行被阻止',async()=>{
 const fetch=mockFetch();render(<AgentStudio item={{...item,spec:{agent:{...item.spec.agent!,handoff:undefined}}}} access={access}/>);
 expect(screen.getByRole('button',{name:'发布'}).hasAttribute('disabled')).toBe(true);
 expect(screen.getByRole('button',{name:'试运行草稿'}).hasAttribute('disabled')).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'交接契约'}));
 for(const name of ['交接目标','输入与上下文','交付物','验收标准','异常处理']){
  const field=screen.getByLabelText(name,{selector:'textarea',exact:true}) as HTMLTextAreaElement;
  expect(field.placeholder).toMatch(/^例如：.+/);
  expect(field.value).toBe('');
 }
 fireEvent.change(screen.getByLabelText('交付物',{exact:true}),{target:{value:'检查报告'}});
 fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));await screen.findByText('已保存 r2');
 const write=fetch.mock.calls.find(([,options])=>options?.method==='PUT');
 expect(JSON.parse(write![1]!.body as string).spec.agent.handoff.deliverables).toBe('检查报告');
 expect(screen.getByRole('button',{name:'发布'}).hasAttribute('disabled')).toBe(true);
 for(const name of ['交接目标','输入与上下文','验收标准','异常处理'])fireEvent.change(screen.getByLabelText(name,{selector:'textarea',exact:true}),{target:{value:'已补齐'+name}});
 fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
 await waitFor(()=>expect(screen.getByRole('button',{name:'发布'}).hasAttribute('disabled')).toBe(false));
 expect(screen.getByRole('button',{name:'试运行草稿'}).hasAttribute('disabled')).toBe(false);
});
function respond(data:unknown,status=200){return new Response(JSON.stringify(data),{status,headers:{'Content-Type':'application/json'}});}
function mockFetch(conflict=false){
 const fetch=vi.fn(async(url:string,options?:RequestInit)=>{
  if(options?.method==='PUT')return conflict?respond({error:'草稿修订已变化'},409):respond({...item,...JSON.parse(options.body as string),revision:2});
  if(url.includes('/versions'))return respond([]);
  if(url.includes('/trials'))return respond({items:[]});
  return respond(item);
 });vi.stubGlobal('fetch',fetch);return fetch;
}
test('独立工作台保存已编辑职责，未保存时阻止试运行和发布',async()=>{
 const fetch=mockFetch();render(<AgentStudio item={item} access={access}/>);
 expect(await screen.findByRole('heading',{name:'Agent 工作台'})).toBeTruthy();
 fireEvent.change(screen.getByLabelText('助手职责'),{target:{value:'新的职责'}});
 expect(screen.getByRole('button',{name:'试运行草稿'}).hasAttribute('disabled')).toBe(true);
 expect(screen.getByRole('button',{name:'发布'}).hasAttribute('disabled')).toBe(true);
 fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
 await screen.findByText('已保存 r2');
 const call=fetch.mock.calls.find(([,options])=>options?.method==='PUT');
 expect(JSON.parse(call![1]!.body as string).spec.agent.instruction).toBe('新的职责');
 expect(JSON.parse(call![1]!.body as string).revision).toBe(1);
});
test('冲突保留本地内容，服务端变化不被覆盖',async()=>{
 mockFetch(true);render(<AgentStudio item={item} access={access}/>);
 fireEvent.change(screen.getByLabelText('助手职责'),{target:{value:'未保存的工作'}});
 fireEvent.click(screen.getByRole('button',{name:'保存草稿'}));
 await screen.findByText('草稿修订已变化');
 expect((screen.getByLabelText('助手职责') as HTMLTextAreaElement).value).toBe('未保存的工作');
 expect(screen.getByRole('button',{name:'查看服务器草稿'})).toBeTruthy();
});
test('查看者只读且不访问连接选择接口',async()=>{
 const fetch=mockFetch();render(<AgentStudio item={{...item,spec:{agent:{...item.spec.agent!,executor:'eino',connection_version:'c'}}}} access={{...access,role:'viewer'}}/>);
 fireEvent.click(screen.getByRole('button',{name:'模型与执行'}));
 await waitFor(()=>expect(fetch).toHaveBeenCalled());
 expect(fetch.mock.calls.some(([url])=>url.includes('/connections'))).toBe(false);
 expect(screen.getByRole('button',{name:'保存草稿'}).hasAttribute('disabled')).toBe(true);
});
test('未保存职责在离开时受保护，拒绝离开保留编辑',()=>{
 mockFetch();vi.spyOn(window,'confirm').mockReturnValue(false);render(<AgentStudio item={item} access={access}/>);
 fireEvent.change(screen.getByLabelText('助手职责'),{target:{value:'尚未保存'}});
 const event=new Event('pangu:before-navigate',{cancelable:true});window.dispatchEvent(event);
 expect(event.defaultPrevented).toBe(true);expect((screen.getByLabelText('助手职责') as HTMLTextAreaElement).value).toBe('尚未保存');
});
test('已保存草稿的试运行结果未知时也阻止直接离开',async()=>{
 vi.stubGlobal('fetch',vi.fn(async(url:string,options?:RequestInit)=>{if(options?.method==='POST')throw Error('断网');return respond(url.includes('/versions')?[]:{items:[]});}));
 vi.spyOn(window,'confirm').mockReturnValue(false);render(<AgentStudio item={item} access={access}/>);
 fireEvent.change(screen.getByLabelText('任务目标'),{target:{value:'检查项目'}});fireEvent.click(screen.getByText('试运行草稿'));
 await screen.findByText('重试同一试运行');
 const event=new Event('pangu:before-navigate',{cancelable:true});window.dispatchEvent(event);expect(event.defaultPrevented).toBe(true);
 const unload=new Event('beforeunload',{cancelable:true});window.dispatchEvent(unload);expect(unload.defaultPrevented).toBe(true);
});
test('恢复版本确认后带当前修订保存新草稿，不改写发布版本',async()=>{
 const version={id:'v1',object_id:'a',kind:'agent',project_id:'p',number:1,source_revision:1,created_at:item.created_at,spec:{agent:{...item.spec.agent!,instruction:'历史职责'}}};
 const fetch=vi.fn(async(url:string,options?:RequestInit)=>options?.method==='PUT'?respond({...item,...JSON.parse(options.body as string),revision:2}):url.includes('/versions')?respond([version]):respond({items:[]}));
 vi.stubGlobal('fetch',fetch);vi.spyOn(window,'confirm').mockReturnValue(true);render(<AgentStudio item={item} access={access}/>);
 fireEvent.click(screen.getByRole('button',{name:'版本'}));await screen.findByRole('button',{name:'恢复此版本为新草稿'});
 fireEvent.click(screen.getByRole('button',{name:'恢复此版本为新草稿'}));await screen.findByRole('button',{name:'确认恢复为新草稿'});
 expect(fetch.mock.calls.filter(([,options])=>options?.method==='PUT')).toHaveLength(0);
 fireEvent.click(screen.getByRole('button',{name:'暂不恢复'}));expect(screen.queryByRole('button',{name:'确认恢复为新草稿'})).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'恢复此版本为新草稿'}));fireEvent.click(screen.getByRole('button',{name:'确认恢复为新草稿'}));await screen.findByText('已保存 r2');
 const writes=fetch.mock.calls.filter(([,options])=>options?.method==='PUT');expect(writes).toHaveLength(1);
 expect(writes[0][0]).toBe('/api/catalog/a/draft');expect(JSON.parse(writes[0][1]!.body as string)).toMatchObject({revision:1,spec:version.spec});
});
