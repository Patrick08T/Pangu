import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,type Access,type Schema} from '../console/components';
import {VersionPicker} from '../catalog/selection';
import {RunInput} from '../onboarding/input';

export function ScheduleEditor({access,plan,version='',saved}:{access:Access;plan?:Schema['Schedule'];version?:string;saved:()=>void}) {
  const [name,setName]=useState(plan?.name??''),[expression,setExpression]=useState(plan?.expression??'0 0 * * *');
  const [kind,setKind]=useState('workflow');
  const [versionID,setVersion]=useState(plan?.workflow.version_id??version),[input,setInput]=useState(plan?.workflow.input??'');
  const [paused,setPaused]=useState(plan?.paused??false),[preview,setPreview]=useState<string[]>([]),action=useAction();
  const fixed=useQuery<Schema['CatalogVersion']>(versionID?`/versions/${encodeURIComponent(versionID)}`:null),item=fixed.data;
  const scoped=item?.id===versionID&&!item.archived&&['agent','workflow'].includes(item.kind);
  const retainSnapshot=!!plan&&paused&&versionID===(plan.workflow.version_id??'')&&input===(plan.workflow.input??'');
  async function save(){
    if(!retainSnapshot&&(!scoped||fixed.loading||fixed.error))throw Error('请选择有效发布版本。');
    const workflow=retainSnapshot?plan!.workflow:{version_id:versionID,project_id:access.project,input,nodes:[]};
    const cron={name,expression,workflow};
    await apiRequest(plan?`/crons/${plan.id}`:'/crons',{method:plan?'PUT':'POST',csrf:access.session.csrf,body:plan?{revision:plan.revision,paused,cron}:cron});if(action.active())saved();
  }
  async function showPreview(){setPreview(await apiRequest<string[]>(`/crons/preview?expression=${encodeURIComponent(expression)}`));}
  return <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(save,'计划已保存');}}>
    <fieldset disabled={action.busy}><legend>{plan?`编辑计划 · 修订 ${plan.revision}`:'新建计划'}</legend>
      <label className="field">计划名称<input required maxLength={128} value={name} onChange={e=>setName(e.target.value)}/><small>支持中文，最多 128 字节。</small></label>
      <label className="field">Cron 表达式（UTC，五个字段）<input required value={expression} onChange={e=>{setExpression(e.target.value);setPreview([]);}}/></label>
      <button type="button" onClick={()=>void action.run(showPreview,'已读取后续触发时间')}>预览后五次 UTC 触发</button>
      {preview.length>0&&<ol className="preview">{preview.map(time=><li key={time}><time>{new Date(time).toISOString()}</time></li>)}</ol>}
      <QueryState {...fixed} retry={fixed.reload}/>
      {scoped&&<p>固定使用{item.kind==='workflow'?'工作流':'助手'} v{item.number}。<a href={`#/catalog/${item.object_id}`}>查看配置</a></p>}
      {item&&!fixed.loading&&!scoped&&<p role="alert">版本不可用于新任务，请重新选择。已有计划仍可保留原输入并暂停。</p>}
      <details open={!versionID}><summary>{versionID?'更换已发布版本':'选择助手或工作流'}</summary>
        <label className="field">类型<select value={kind} onChange={e=>setKind(e.target.value)}><option value="workflow">工作流</option><option value="agent">助手</option></select></label>
        <VersionPicker key={kind} kind={kind} value={versionID} disabled={action.busy} change={next=>{setVersion(next?.id??'');setInput('');}}/>
      </details>
      {plan&&!plan.workflow.version_id&&<p className="notice">此计划使用直接定义。修改前请在画布发布工作流，再选择发布版本。</p>}
      {scoped&&<RunInput key={item.id} version={item} value={input} change={setInput} disabled={action.busy}/>}
      {plan&&<label className="check"><input type="checkbox" checked={paused} onChange={e=>setPaused(e.target.checked)}/>暂停计划</label>}
      <p className="muted">保存使用明确版本，不自动升级。已有活动运行时跳过触发，漏跑不补；暂停和删除不取消已接受的运行。{!plan&&'创建成功后计划立即启用。'}</p>
      <button className="primary" disabled={!retainSnapshot&&(!scoped||fixed.loading||!!fixed.error)}>保存计划</button>
    </fieldset><ActionNotice {...action}/>
  </form>;
}
