import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Factory} from './workflow';
import type {Schema} from '../console/components';

vi.mock('./preview',()=>({Proposal:({turn,apply,disabled}:{turn:Schema['FactoryTurn'];apply:()=>void;disabled:boolean})=><><p>{turn.reply?.message}</p>{turn.status==='ready'&&<button disabled={disabled} onClick={apply}>确认创建并发布</button>}{turn.result&&<a href={`#/onboarding/${turn.result.agents[0].id}`}>运行此助手</a>}</>}));
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
const session:Schema['FactorySession']={id:'s',project_id:'p',created_by:'u',title:'新对话',revision:1,updated_at:'2026-10-27',turns:[]};
const response=(body:unknown,status=200)=>new Response(JSON.stringify(body),{status});
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
function service(current:()=>Schema['FactorySession'],handle:(url:string,options:RequestInit)=>Promise<Response>){
 const fetcher=vi.fn(async(url:string,options:RequestInit)=>{
  if(url.includes('/connections/choices'))return response({items:[{id:'c',name:'Codex 登录',model:'',number:1}]});
  if(url.includes('/catalog?'))return response({items:[]});
  if(url.includes('/runs?'))return response({items:[]});
  if(url.includes('/factory/sessions?'))return response({items:[current()]});
  if(url==='/api/factory/sessions/s')return response(current());
  return handle(url,options);
 });vi.stubGlobal('fetch',fetcher);return fetcher;
}

test.each([false,true])('发送需求遇到断网或无效回执时重试使用同一消息 ID（无效回执=%s）',async invalid=>{
 const bodies:unknown[]=[];
 const fetcher=service(()=>session,async(url,options)=>{if(url.endsWith('/messages')){bodies.push(JSON.parse(String(options.body)));if(bodies.length===1){if(invalid)return response({});throw new TypeError('offline');}return response({request_id:'00000000-0000-0000-0000-000000000001',status:'pending'});}throw Error(url);});
 render(<Factory access={access} id="s"/>);
 fireEvent.change(await screen.findByLabelText('告诉工厂助手你的需求'),{target:{value:'创建产品经理'}});
 await waitFor(()=>expect((screen.getByRole('button',{name:'发送需求'}) as HTMLButtonElement).disabled).toBe(false));
 fireEvent.click(screen.getByRole('button',{name:'发送需求'}));
 fireEvent.click(await screen.findByRole('button',{name:'重试同一条消息'}));
 await waitFor(()=>expect(bodies).toHaveLength(2));expect(bodies[0]).toEqual(bodies[1]);
 expect(fetcher.mock.calls.some(([url])=>url.endsWith('/run')||url.endsWith('/apply'))).toBe(false);
});

test('必须确认候选才批量发布，发布后只提供运行入口',async()=>{
 let current:Schema['FactorySession']={...session,revision:3,turns:[{id:'t',input:{id:'t',revision:1,message:'写作助手',mode:'build',connection_version:'c'},skill_digest:'abc',status:'ready',created_at:'2026-10-27',reply:{message:'请预览',proposal:{name:'写作',agents:[]}}}]};
 const result={agents:[{id:'00000000-0000-0000-0000-000000000010',object_id:'00000000-0000-0000-0000-000000000011',kind:'agent',project_id:'p',number:1,source_revision:1,spec:{},archived:false,created_at:'2026-10-27'}]};
 const fetcher=service(()=>current,async(url)=>{if(url.endsWith('/apply')){current={...current,revision:4,turns:[{...current.turns[0],status:'applied',result}]};return response(result);}throw Error(url);});
 render(<Factory access={{...access,catalogManager:true}} id="s"/>);
 const button=await screen.findByRole('button',{name:'确认创建并发布'});
 expect(fetcher.mock.calls.some(([url])=>url.endsWith('/apply'))).toBe(false);
 fireEvent.click(button);await screen.findByRole('link',{name:'运行此助手'});
 expect(fetcher.mock.calls.filter(([url])=>url.endsWith('/apply'))).toHaveLength(1);
 expect(fetcher.mock.calls.some(([url])=>url.endsWith('/run'))).toBe(false);
});

test('工厂从共享目录选择模型与配置，任务证据仍按执行项目读取',async()=>{
 const fetcher=service(()=>session,async url=>{throw Error(url);});
 render(<Factory access={access} id="s"/>);
 fireEvent.change(await screen.findByLabelText('工作方式'),{target:{value:'optimize'}});
 await waitFor(()=>expect(fetcher.mock.calls.some(([url])=>url==='/api/catalog?offset=0')).toBe(true));
 expect(fetcher.mock.calls.some(([url])=>url==='/api/connections/choices')).toBe(true);
 expect(fetcher.mock.calls.some(([url])=>url==='/api/runs?project_id=p&limit=100')).toBe(true);
 expect(fetcher.mock.calls.some(([url])=>/\/(catalog|connections\/choices)\?.*project_id/.test(url))).toBe(false);
});

test('项目操作者可生成方案，发布共享配置需要配置管理权限',async()=>{
 const current:Schema['FactorySession']={...session,turns:[{id:'t',input:{id:'t',revision:1,message:'需求',mode:'build',connection_version:'c'},skill_digest:'abc',status:'ready',created_at:'2026-10-27',reply:{message:'请预览',proposal:{name:'写作',agents:[]}}}]};
 service(()=>current,async url=>{throw Error(url);});
 render(<Factory access={access} id="s"/>);
 expect((await screen.findByRole('button',{name:'确认创建并发布'}) as HTMLButtonElement).disabled).toBe(true);
});
