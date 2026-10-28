import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,JSONField,QueryState,type Access,type Schema} from '../console/components';
import {RequestReceipt,receipt} from './submission';

export function ForkRun({run,access}:{run:Schema['RunDetails'];access:Access}) {
 const [open,setOpen]=useState(false),[text,setText]=useState(JSON.stringify(run.definition,null,2));
 const choices=run.nodes.filter(n=>n.executor==='codex'&&n.status==='succeeded');
 const [checkpoint,setCheckpoint]=useState(choices[0]?.node_id??''),[preview,setPreview]=useState<Schema['ForkPreview']>();
 const [checked,setChecked]=useState(''),[confirmed,setConfirmed]=useState(false),[request,setRequest]=useState('');
 const action=useAction(),check=useAction(),key=useRef(crypto.randomUUID());
 const fingerprint=JSON.stringify([checkpoint,text]);
 const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500||action.error.status===409);
 const disabled=action.busy||check.busy||uncertain;
 function body(){return {checkpoint_node:checkpoint,workflow:JSON.parse(text)};}
 async function inspect(){
  setChecked('');setConfirmed(false);setPreview(undefined);
  const result=await apiRequest<Schema['ForkPreview']>(`/runs/${run.id}/fork/preview`,{method:'POST',csrf:access.session.csrf,body:body()});
  if(check.active()){setPreview(result);setChecked(fingerprint);}
 }
 async function submit(){
  if(checked!==fingerprint||!confirmed)throw Error('请重新预览并确认返工范围');
  const result=await apiRequest<Schema['Submission']>(`/runs/${run.id}/fork`,{method:'POST',csrf:access.session.csrf,key:key.current,body:body()});
  if(action.active())setRequest(receipt(result));
 }
 return <section><button disabled={!choices.length} onClick={()=>setOpen(!open)}>检查点返工</button>{open&&<>
  <p>创建独立运行，保留原历史。首版只复用串行、无条件的 Codex 成功前缀；已变更节点及其后续重新执行。全局输入改变时全部重做。文件检查点不能撤销模型调用、外部写入等副作用。</p>
  {request?<RequestReceipt id={request}/>:<form className="editor" onSubmit={e=>{e.preventDefault();void action.run(submit);}}>
   <label className="field">来源成功检查点<select value={checkpoint} disabled={disabled} onChange={e=>{setCheckpoint(e.target.value);setConfirmed(false);}}>{choices.map(node=><option key={node.node_id} value={node.node_id}>{node.node_id}</option>)}</select></label>
   <JSONField label="返工定义（JSON）" value={text} disabled={disabled} onChange={value=>{setText(value);setConfirmed(false);}}/>
   <button type="button" disabled={disabled} onClick={()=>void check.run(inspect)}>预览返工范围</button>
   {preview&&checked===fingerprint&&<div className="notice"><p>将复用：{preview.reused_nodes.join('、')||'无'}</p><p>将执行：{preview.execute_nodes.join('、')||'无'}</p><p>全局输入{preview.input_changed?'已变化，旧结果全部失效':'未变化'}</p>
    <label><input type="checkbox" checked={confirmed} disabled={disabled} onChange={e=>setConfirmed(e.target.checked)}/>我确认创建独立运行，并承担新增调用消耗</label></div>}
   {uncertain&&<p className="notice">提交结果未知，定义已冻结；重试沿用同一个请求键。</p>}
   <button className="primary" disabled={action.busy||check.busy||!confirmed||checked!==fingerprint}>{uncertain?'重试同一返工提交':'创建返工运行'}</button>
   <ActionNotice {...check}/><ActionNotice {...action}/>
  </form>}
 </>}</section>;
}

export function Inputs({run,stopped}:{run:string;stopped:boolean}) {
 const query=useQuery<Schema['InputPreview'][]>(`/runs/${run}/inputs`,stopped?0:5000);
 const labels={ready:'输入已就绪',pending:'等待上游输出',skipped:'条件未命中或依赖已跳过',invalid:'输入错误'};
 return <details><summary>节点输入预览</summary><QueryState {...query} retry={query.reload}/>
  <p>只展示当前可确定的结构化参数；等待中的上游输出不会被猜测。</p>
  {query.data?.map(node=><article key={node.node_id}><button onClick={()=>document.getElementById(`node-${node.node_id}`)?.focus()}>{node.node_id}</button> · {labels[node.state]}
   {node.error&&<p className="error-text">{node.error}</p>}<pre>{JSON.stringify(node.parameters,null,2)}</pre></article>)}
 </details>;
}
