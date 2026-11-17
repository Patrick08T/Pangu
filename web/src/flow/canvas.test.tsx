import {act,cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {FlowCanvas} from './canvas';
import type {Flow,Versions} from './model';

let props:any;
vi.mock('@xyflow/react',async original=>{
  const actual=await original<typeof import('@xyflow/react')>();
  return {...actual,ReactFlow:(value:any)=>{props=value;const Card=value.nodeTypes.assistant;return <div>{value.nodes.map((node:any)=><Card key={node.id} {...node}/>)}</div>;},Handle:()=>null,Background:()=>null,Controls:()=>null,MiniMap:()=>null};
});
afterEach(cleanup);
const flow:Flow={nodes:[{id:'a',agent_version:'v'},{id:'b',agent_version:'v'}],editor:{nodes:{a:{x:1,y:2},b:{x:3,y:4}},viewport:{x:0,y:0,zoom:1}}};
test('画布按实际绑定版本显示角色，步骤改名不覆盖角色，换版本后更新角色',()=>{
  const versions:Versions={v:{id:'v',object_id:'agent',kind:'agent',number:2,source_revision:1,created_at:'2026-10-30',archived:false,spec:{agent:{id:'developer',executor:'codex',instruction:'实现需求'}}}};
  const namedVersions={v:{...versions.v,agentName:'后端开发工程师'},qa:{...versions.v,id:'qa',agentName:'独立 QA 测试工程师'}};
  const selected:Flow={nodes:[flow.nodes[0]],editor:{nodes:{a:{x:0,y:0,name:'本轮迭代实现'}}}};
  const common={versions:namedVersions,select:()=>{},change:()=>{}};
  const {rerender}=render(<FlowCanvas flow={selected} {...common}/>);
  expect(screen.getByText('本轮迭代实现')).toBeTruthy();
  expect(screen.getByText('Agent 角色：后端开发工程师')).toBeTruthy();
  expect(screen.getByText('Codex · v2')).toBeTruthy();
  rerender(<FlowCanvas flow={{...selected,nodes:[{id:'a',agent_version:'qa'}]}} {...common}/>);
  expect(screen.getByText('Agent 角色：独立 QA 测试工程师')).toBeTruthy();
  expect(screen.queryByText('Agent 角色：后端开发工程师')).toBeNull();
});
test('一次批量移动提交全部坐标，不覆盖另一节点',()=>{
  const change=vi.fn();render(<FlowCanvas flow={flow} versions={{}} select={()=>{}} change={change}/>);
  act(()=>props.onNodesChange([{id:'a',type:'position',position:{x:20,y:30},dragging:false},{id:'b',type:'position',position:{x:50,y:60},dragging:false}]));
  expect(change).toHaveBeenCalledTimes(1);expect(change.mock.calls[0][0].editor.nodes).toMatchObject({a:{x:20,y:30},b:{x:50,y:60}});
});
test('撤销后画布视图同步保存状态',()=>{
  const common={versions:{},select:()=>{},change:()=>{}};
  const {rerender}=render(<FlowCanvas flow={flow} {...common}/>);
  const moved={...flow,editor:{...flow.editor,viewport:{x:40,y:20,zoom:0.5}}};
  rerender(<FlowCanvas flow={moved} {...common}/>);expect(props.viewport).toEqual(moved.editor.viewport);
  rerender(<FlowCanvas flow={flow} {...common}/>);expect(props.viewport).toEqual(flow.editor!.viewport);
});
test('删除节点和关联连线只提交一次，节点不会被旧状态恢复',()=>{
  const change=vi.fn();const linked={...flow,nodes:[flow.nodes[0],{...flow.nodes[1],depends_on:['a']}]};
  render(<FlowCanvas flow={linked} versions={{}} select={()=>{}} change={change}/>);
  act(()=>props.onDelete({nodes:[{id:'a'}],edges:[{source:'a',target:'b'}]}));
  expect(change).toHaveBeenCalledTimes(1);expect(change.mock.calls[0][0].nodes).toEqual([{...flow.nodes[1],depends_on:[]}]);
});
test('用户点击查看全图会保存最终视图，供刷新和撤销使用',async()=>{
  const change=vi.fn();render(<FlowCanvas flow={flow} versions={{}} select={()=>{}} change={change}/>);
  const view={x:30,y:40,zoom:0.6};act(()=>props.onInit({fitView:async()=>true,getViewport:()=>view}));
  fireEvent.click(screen.getByRole('button',{name:'查看全图'}));
  await waitFor(()=>expect(change).toHaveBeenCalledOnce());expect(change.mock.calls[0][0].editor.viewport).toEqual(view);
});
test('多选节点不会被侧栏选择覆盖，移动后保留选择和测量尺寸',()=>{
  const select=vi.fn(),common={versions:{},select,change:()=>{},selected:'a'};
  const {rerender}=render(<FlowCanvas flow={flow} {...common}/>);
  act(()=>{props.onNodesChange([{id:'a',type:'select',selected:true},{id:'b',type:'select',selected:true},{id:'a',type:'dimensions',dimensions:{width:240,height:90}}]);props.onNodeClick({ctrlKey:true},{id:'b'});});
  expect(select).not.toHaveBeenCalled();
  rerender(<FlowCanvas flow={{...flow,editor:{...flow.editor,nodes:{a:{x:8,y:9},b:{x:10,y:11}}}}} {...common}/>);
  expect(props.nodes.map((node:any)=>node.selected)).toEqual([true,true]);expect(props.nodes[0].measured).toEqual({width:240,height:90});
});
