import React,{useRef,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {FreeLayoutEditorProvider,EditorRenderer,WorkflowNodeRenderer} from '@flowgram.ai/free-layout-editor';
import '@flowgram.ai/free-layout-editor/index.css';
import {fixture} from './fixture.mjs';
import {toGraph,fromGraph} from './bridge.mjs';
import './style.css';

function Assistant({node}){
  return <WorkflowNodeRenderer node={node} className="assistant"><strong>{node.getExtInfo().title}</strong><p>固定助手版本 · Pangu 执行</p></WorkflowNodeRenderer>;
}
function App(){
  const ctx=useRef(),[result,setResult]=useState('等待验证'),[saved,setSaved]=useState(fixture);
  function check(){
    try{const next=fromGraph(saved,ctx.current.document.toJSON());setResult(JSON.stringify(next)===JSON.stringify(saved)?'往返验证通过：Pangu 配置完整保留':'已读取画布修改；业务配置完整保留');setSaved(next);}
    catch(error){setResult(`验证失败：${error.message}`);}
  }
  return <main><h1>FlowGram 接入实验</h1><p>独立 React 19 + Vite 环境，仅验证画布，不调用模型或调度任务。</p>
    <div className="toolbar"><button onClick={check}>保存并验证往返</button><button onClick={()=>{ctx.current.document.fromJSON(toGraph(saved));setResult('已重新加载保存的数据');}}>重新加载</button><button onClick={()=>ctx.current.document.fitView()}>适应画布</button><output>{result}</output></div>
    <div className="canvas"><FreeLayoutEditorProvider initialData={toGraph(fixture)} nodeEngine={{enable:false}} nodeRegistries={[{type:'assistant',meta:{defaultPorts:[{type:'input'},{type:'output'}]}}]} materials={{renderDefaultNode:Assistant}} onInit={context=>{ctx.current=context;}}><EditorRenderer style={{width:'100%',height:'100%'}}/></FreeLayoutEditorProvider></div>
  </main>;
}
class ExperimentBoundary extends React.Component{
  state={error:null};
  static getDerivedStateFromError(error){return {error};}
  render(){return this.state.error?<main><h1>FlowGram 初始化失败</h1><pre>{this.state.error.message}</pre><a href="?strict=off">仅在实验中关闭严格模式，确认问题范围</a></main>:this.props.children;}
}
const app=new URLSearchParams(location.search).get('strict')==='off'?<App/>:<React.StrictMode><App/></React.StrictMode>;
createRoot(document.getElementById('root')).render(<ExperimentBoundary>{app}</ExperimentBoundary>);
