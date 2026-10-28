import {useEffect,useRef,useState} from 'react';
import {ReactFlow,Background,Controls,ControlButton,MiniMap,Handle,Position,MarkerType,applyNodeChanges,applyEdgeChanges,type Node,type Edge,type NodeProps,type ReactFlowInstance} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {arrange,connect,disconnect,display,label,removeNodes,type Flow,type Versions} from './model';

type CanvasNode=Node<{label:string;executor:string;version?:number;condition:boolean;approval:boolean;problem:boolean},'assistant'>;
function AssistantNode({data,selected}:NodeProps<CanvasNode>){
  return <div className={`flow-card ${selected?'selected':''} ${data.problem?'invalid':''}`}>
    <Handle type="target" position={Position.Left} aria-label="前置步骤连接点"/>
    <div className="flow-card-title"><span className={`flow-kind ${data.executor}`}>{data.executor==='codex'?'C':'E'}</span><strong>{data.label}</strong></div>
    <small>{data.executor==='codex'?'Codex':data.executor==='eino'?'Eino':'读取助手…'}{data.version?` · v${data.version}`:''}</small>
    {(data.condition||data.approval)&&<div className="flow-badges">{data.condition&&<span>条件执行</span>}{data.approval&&<span>人工审批</span>}</div>}
    {data.problem&&<small className="error-text">需要完善配置</small>}
    <Handle type="source" position={Position.Right} aria-label="后续步骤连接点"/>
  </div>;
}
const nodeTypes={assistant:AssistantNode};
export function FlowCanvas({flow,versions,selected,select,change,disabled=false,problems=[]}:{flow:Flow;versions:Versions;selected?:string;select:(id?:string)=>void;change:(flow:Flow)=>void;disabled?:boolean;problems?:string[]}) {
  const [nodes,setNodes]=useState<CanvasNode[]>([]),[edges,setEdges]=useState<Edge[]>([]),[instance,setInstance]=useState<ReactFlowInstance<CanvasNode>>();
  const initialViewport=useRef({x:0,y:0,zoom:1});
  const [viewport,setViewport]=useState(flow.editor?.viewport??initialViewport.current);
  useEffect(()=>setViewport(flow.editor?.viewport??initialViewport.current),[flow.editor?.viewport]);
  async function moveView(operation:'fitView'|'zoomIn'|'zoomOut'){
    if(!instance)return;if(!flow.editor?.viewport)initialViewport.current=instance.getViewport();
    await instance[operation]({padding:0.2,maxZoom:1});
    if(!disabled)change({...flow,editor:{...flow.editor,viewport:instance.getViewport()}});
  }
  useEffect(()=>{
    const automatic=arrange(flow).editor!.nodes!;
    setNodes(current=>flow.nodes.map(step=>({ ...current.find(n=>n.id===step.id),id:step.id,type:'assistant',position:flow.editor?.nodes?.[step.id]??automatic[step.id],selected:current.find(n=>n.id===step.id)?.selected??step.id===selected,
      data:{label:label(flow,step.id),executor:versions[step.agent_version]?.spec.agent?.executor??'',version:versions[step.agent_version]?.number,condition:!!step.when,approval:!!(step.approval||versions[step.agent_version]?.spec.agent?.approval),problem:problems.includes(step.id)}})));
    setEdges(flow.nodes.flatMap(step=>(step.depends_on??[]).filter(id=>flow.nodes.some(n=>n.id===id)).map(id=>({id:`${id}:${step.id}`,source:id,target:step.id,type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed},ariaLabel:`${label(flow,id)} 到 ${label(flow,step.id)}`}))));
  },[flow,versions,problems.join(',')]);
  useEffect(()=>setNodes(current=>current.map(node=>({...node,selected:node.id===selected}))),[selected]);
  return <div className="flow-canvas" aria-label="流程画布"><ReactFlow<CanvasNode> nodes={nodes} edges={edges} nodeTypes={nodeTypes} minZoom={0.1} maxZoom={2} fitView={!flow.editor?.viewport} fitViewOptions={{maxZoom:1,padding:0.2}} viewport={viewport} onViewportChange={setViewport} onInit={setInstance}
    onNodesChange={changes=>{setNodes(current=>applyNodeChanges(changes,current));if(disabled)return;const next=changes.reduce((value,item)=>item.type==='position'&&item.position&&item.dragging===false?display(value,item.id,item.position):value,flow);if(next!==flow)change(next);}}
    onEdgesChange={changes=>setEdges(current=>applyEdgeChanges(changes,current))}
    onNodeClick={(event,node)=>{if(!event.ctrlKey&&!event.metaKey)select(node.id);}} onPaneClick={()=>select(undefined)}
    onConnect={edge=>{if(!disabled)change(connect(flow,edge.source,edge.target));}}
    isValidConnection={edge=>{try{connect(flow,edge.source,edge.target);return true;}catch{return false;}}}
    onDelete={({nodes:removed,edges:links})=>{if(!disabled)change(links.reduce((value,edge)=>disconnect(value,edge.source,edge.target),removeNodes(flow,removed.map(n=>n.id))));}}
    onMoveStart={(_,view)=>{if(!flow.editor?.viewport)initialViewport.current=view;}}
    onMoveEnd={(event,viewport)=>{if(event&&!disabled)change({...flow,editor:{...flow.editor,viewport}});}}
    nodesDraggable={!disabled} nodesConnectable={!disabled} edgesReconnectable={false} deleteKeyCode={disabled?null:['Backspace','Delete']} nodesFocusable edgesFocusable
    ariaLabelConfig={{'controls.zoomIn.ariaLabel':'放大','controls.zoomOut.ariaLabel':'缩小','controls.fitView.ariaLabel':'适配画布','minimap.ariaLabel':'流程缩略图'}}>
    <Background color="#cbd5e1" gap={24}/><Controls showInteractive={false} showZoom={false} showFitView={false}>
      <ControlButton aria-label="放大" disabled={viewport.zoom>=2} onClick={()=>void moveView('zoomIn')}>＋</ControlButton>
      <ControlButton aria-label="缩小" disabled={viewport.zoom<=0.1} onClick={()=>void moveView('zoomOut')}>−</ControlButton>
      <ControlButton aria-label="适配画布" onClick={()=>void moveView('fitView')}>⤢</ControlButton>
    </Controls><MiniMap nodeColor="#d4deed"/>
  </ReactFlow>{!nodes.length&&<div className="flow-empty"><strong>从一个助手开始</strong><p>点击左侧助手添加步骤，连接步骤安排执行顺序。</p></div>}
    <button type="button" className="flow-fit" onClick={()=>void moveView('fitView')}>查看全图</button>
  </div>;
}
