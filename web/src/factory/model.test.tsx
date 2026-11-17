import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Factory} from './workflow';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
const access={project:'p',role:'operator',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}};
function setup(turns:unknown[]=[]){
 const requests:Record<string,unknown>[]=[];
 vi.stubGlobal('fetch',vi.fn(async(url:string,options?:RequestInit)=>{
  if(url.includes('/connections/choices'))return Response.json({items:[{id:'c',name:'Codex 账号',model:'',number:1,provider:'codex'},{id:'api',name:'API 模型',model:'fixed',number:1,provider:'openai'}]});
  if(url.includes('/models/codex'))return Response.json({items:[{id:'chosen',name:'测试模型',reasoning_levels:['low','high'],default_reasoning:'low',fast:true}]});
  const session={id:'s',project_id:'p',created_by:'u',title:'对话',revision:1,updated_at:'2026-10-30',turns};
  if(url.includes('/factory/sessions?'))return Response.json({items:[session]});
  if(url==='/api/factory/sessions/s')return Response.json(session);
  if(url.endsWith('/messages')){requests.push(JSON.parse(String(options?.body)));throw new TypeError('offline');}
  throw Error(url);
 }));
 render(<Factory access={access} id="s"/>);return requests;
}

test('工厂消息发送所选模型强度和 Fast，断网重试保持原配置',async()=>{
 const requests=setup();
 fireEvent.change(await screen.findByLabelText('告诉工厂助手你的需求'),{target:{value:'创建助手'}});
 await screen.findByRole('option',{name:'测试模型'});
 fireEvent.change(screen.getByLabelText('Codex 模型'),{target:{value:'chosen'}});
 fireEvent.change(screen.getByLabelText('推理强度'),{target:{value:'high'}});
 fireEvent.change(screen.getByLabelText('Fast 模式'),{target:{value:'false'}});
 fireEvent.click(screen.getByRole('button',{name:'发送需求'}));
 fireEvent.click(await screen.findByRole('button',{name:'重试同一条消息'}));
 await waitFor(()=>expect(requests).toHaveLength(2));
 expect(requests[0]).toMatchObject({connection_version:'c',model:'chosen',reasoning_effort:'high',fast:false});
 expect(requests[1]).toEqual(requests[0]);
 expect((screen.getByLabelText('Codex 模型') as HTMLSelectElement).disabled).toBe(true);
});

test('切换到 API 连接清除 Codex 选项',async()=>{
 const requests=setup();
 await screen.findByRole('option',{name:'测试模型'});
 fireEvent.change(screen.getByLabelText('Codex 模型'),{target:{value:'chosen'}});
 fireEvent.change(screen.getByLabelText('推理强度'),{target:{value:'high'}});
 fireEvent.change(screen.getByLabelText('模型连接'),{target:{value:'api'}});
 expect(screen.queryByLabelText('Fast 模式')).toBeNull();
 fireEvent.change(screen.getByLabelText('告诉工厂助手你的需求'),{target:{value:'创建助手'}});
 fireEvent.click(screen.getByRole('button',{name:'发送需求'}));
 await waitFor(()=>expect(requests).toHaveLength(1));
 expect(requests[0]).not.toHaveProperty('reasoning_effort');expect(requests[0]).not.toHaveProperty('fast');
});

test('历史显示消息配置并恢复为下次发送的初始选择',async()=>{
 setup([{id:'t',input:{id:'t',revision:1,message:'旧消息',mode:'build',connection_version:'c',model:'chosen',reasoning_effort:'high',fast:false},skill_digest:'digest',status:'failed',error:'模型暂时不可用',created_at:'2026-10-30'}]);
 await screen.findByText(/chosen · 推理：high · Fast：关闭/);
 await screen.findByRole('option',{name:'测试模型'});
 expect((screen.getByLabelText('Codex 模型') as HTMLSelectElement).value).toBe('chosen');
 expect((screen.getByLabelText('推理强度') as HTMLSelectElement).value).toBe('high');
});

test('切回默认模型清除强度与 Fast，保持继承设置',async()=>{
 const requests=setup();
 await screen.findByRole('option',{name:'测试模型'});
 fireEvent.change(screen.getByLabelText('Codex 模型'),{target:{value:'chosen'}});
 fireEvent.change(screen.getByLabelText('推理强度'),{target:{value:'high'}});
 fireEvent.change(screen.getByLabelText('Fast 模式'),{target:{value:'true'}});
 fireEvent.change(screen.getByLabelText('Codex 模型'),{target:{value:''}});
 expect((screen.getByLabelText('推理强度') as HTMLSelectElement).disabled).toBe(true);
 fireEvent.change(screen.getByLabelText('告诉工厂助手你的需求'),{target:{value:'创建助手'}});
 fireEvent.click(screen.getByRole('button',{name:'发送需求'}));
 await waitFor(()=>expect(requests).toHaveLength(1));
 expect(requests[0]).not.toHaveProperty('model');
 expect(requests[0]).not.toHaveProperty('reasoning_effort');
 expect(requests[0]).not.toHaveProperty('fast');
});
