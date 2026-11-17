import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,QueryState,isManager,type Access,type Schema} from '../console/components';
import {ExecutionPlan} from './plan';
import {ReviewForm} from './review';

type Props={gate:Schema['ApprovalGate'];access:Access;reload:()=>void};
export function EvidenceActions(props:Props){
 const repair=props.gate.repair;
 return <>{repair?<RepairChoice key={repair.hash} {...props} repair={repair}/>:<><p className="notice">执行配置需要修正时，应先准备修正方案；仅重新检查不会修改执行图。</p><ReviewForm {...props}/></>}
  {isManager(props.access)&&<details><summary>{repair?'修订续跑方案':'准备修正版本与续跑方案'}</summary><RepairPreparation {...props}/></details>}</>;
}

export function RepairChoice({gate,repair,access,reload}:Props&{repair:Schema['EvidenceRepair']}){
 const [mode,setMode]=useState(''),action=useAction();
 const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
 const conflict=action.error instanceof APIError&&[409,410].includes(action.error.status);
 async function approve(){await apiRequest(`/approvals/${gate.id}`,{method:'PUT',csrf:access.session.csrf,body:{subject_hash:gate.subject_hash,repair_hash:repair.hash,decision:'approved',reason:'',items:[]}});reload();}
 return <section aria-label="阻塞处理方式"><h2>选择处理方式</h2>
  <p>已准备具体修正方案。批准后发布新版本并定向续跑；保留原结论，最终由独立验收决定。</p>
  <fieldset disabled={action.busy||uncertain||conflict}><legend>后续处理</legend>
   <label><input type="radio" name="repair-mode" checked={mode==='repair'} onChange={()=>setMode('repair')}/>保留历史结论，发布修正版本，复用成果并补跑受影响步骤，再独立验收（推荐）</label>
   <label><input type="radio" name="repair-mode" checked={mode==='evidence'} onChange={()=>setMode('evidence')}/>已有新增证据或判断纠正，只做定向复验</label>
  </fieldset>
  <p>推荐理由：当前问题需要落实执行图修正，重复核验相同材料不能消除差异。</p>
  <p>{repair.summary}</p><p>将复用：{repair.reused_nodes.join('、')}</p><p>将补跑：{repair.execute_nodes.join('、')}</p>
  <details><summary>查看 {repair.execution.changes.length} 项执行图差异</summary><ExecutionPlan plan={repair.execution}/></details>
  {mode==='evidence'?<ReviewForm gate={gate} access={access} reload={reload}/>:<button className="primary" disabled={mode!=='repair'||action.busy||conflict} onClick={()=>void action.run(approve,'修正方案已批准，正在发布并续跑')}>{uncertain?'重试同一修正批准':'批准修正版本并续跑'}</button>}
  {conflict&&<button onClick={reload}>重新读取修正方案</button>}<ActionNotice {...action}/>
 </section>;
}

function RepairPreparation({gate,access,reload}:Props){
 const [object,setObject]=useState(''),[checkpoint,setCheckpoint]=useState(''),[summary,setSummary]=useState(''),[offset,setOffset]=useState(0);
 const choices=useQuery<Schema['CatalogPage']>(`/catalog?kind=workflow&offset=${offset}`),run=useQuery<Schema['RunDetails']>(`/runs/${gate.run_id}`),action=useAction();
 const selected=choices.data?.items.find(c=>c.id===object);
 const nodes=[...new Set(run.data?.nodes.filter(n=>n.status==='succeeded'&&n.executor==='codex').map(n=>n.node_id)??[])];
 async function prepare(){await apiRequest(`/approvals/${gate.id}/repair`,{method:'POST',csrf:access.session.csrf,body:{subject_hash:gate.subject_hash,object_id:object,revision:selected!.revision,checkpoint_node:checkpoint,summary}});reload();}
 return <div className="editor"><p>先在<a href="#/workflow">工作流</a>中准备修正草稿，再选择可复用的成功检查点。准备方案不会发布版本或调用模型。</p>
  <QueryState {...choices} retry={choices.reload}/><QueryState {...run} retry={run.reload}/>
  <label className="field">修正工作流草稿<select value={object} onChange={e=>setObject(e.target.value)}><option value="">请选择</option>{choices.data?.items.filter(c=>!c.archived).map(c=><option key={c.id} value={c.id}>{c.name} · 修订 {c.revision}</option>)}</select></label>
  <div className="toolbar"><button disabled={offset===0} onClick={()=>{setOffset(offset-100);setObject('');}}>上一页草稿</button><button disabled={choices.data?.next_offset===null||choices.data?.next_offset===undefined} onClick={()=>{setOffset(choices.data!.next_offset!);setObject('');}}>下一页草稿</button></div>
  <label className="field">复用到成功检查点<select value={checkpoint} onChange={e=>setCheckpoint(e.target.value)}><option value="">请选择</option>{nodes.map(n=><option key={n}>{n}</option>)}</select></label>
  <label className="field">修正内容与范围说明<textarea value={summary} maxLength={4000} onChange={e=>setSummary(e.target.value)}/></label>
  <button disabled={!selected||!checkpoint||!summary.trim()||action.busy} onClick={()=>void action.run(prepare,'修正方案已准备，请检查差异并选择处理方式')}>生成待审批修正方案</button><ActionNotice {...action}/>
 </div>;
}
