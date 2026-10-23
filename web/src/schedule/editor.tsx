import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,JSONField,type Access,type Schema} from '../console/components';

export function ScheduleEditor({access,plan,version='',saved}:{access:Access;plan?:Schema['Schedule'];version?:string;saved:()=>void}) {
  const [name,setName]=useState(plan?.name??''),[expression,setExpression]=useState(plan?.expression??'0 0 * * *');
  const [mode,setMode]=useState(plan&&!plan.workflow.version_id?'json':'version');
  const [versionID,setVersion]=useState(plan?.workflow.version_id??version),[input,setInput]=useState(plan?.workflow.input??'');
  const [definition,setDefinition]=useState(JSON.stringify(plan?.workflow??{project_id:access.project,input:'',nodes:[]},null,2));
  const [paused,setPaused]=useState(plan?.paused??false),[preview,setPreview]=useState<string[]>([]),action=useAction();
  async function save(){
    const workflow=mode==='version'?{version_id:versionID,project_id:access.project,input,nodes:[]}:JSON.parse(definition) as Schema['Workflow'];
    if(workflow.project_id!==access.project)throw Error('计划必须绑定当前项目。');
    if(mode==='version'&&!versionID.trim())throw Error('请填写已发布的项目版本 ID。');
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
      <label className="field">绑定方式<select value={mode} onChange={e=>setMode(e.target.value)}><option value="version">已发布版本</option><option value="json">JSON 编排</option></select></label>
      {mode==='version'?<><label className="field">固定版本 ID<input required value={versionID} onChange={e=>setVersion(e.target.value)}/></label><label className="field">运行输入<textarea rows={8} value={input} onChange={e=>setInput(e.target.value)}/></label></>:<JSONField label="编排 JSON" value={definition} onChange={setDefinition}/>}
      {plan&&<label className="check"><input type="checkbox" checked={paused} onChange={e=>setPaused(e.target.checked)}/>暂停计划</label>}
      <p className="muted">保存使用明确版本，不自动升级。已有活动运行时跳过触发，漏跑不补；暂停和删除不取消已接受的运行。{!plan&&'创建成功后计划立即启用。'}</p>
      <button className="primary">保存计划</button>
    </fieldset><ActionNotice {...action}/>
  </form>;
}
