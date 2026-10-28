import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {VersionPicker} from '../catalog/selection';
import {ActionNotice,Page,Pager,QueryState,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';
import {receipt} from '../runs/submission';
import {decisions} from './feedback';

export function Cases({access}:{access:Access}) {
  const [offset,setOffset]=useState(0),query=useQuery<Schema['EvaluationCasePage']>(`/evaluations/cases?project_id=${access.project}&offset=${offset}`);
  return <Page title="固定评测样本" description="用固定输入与验收标准比较已发布版本。样本不能编辑；修改标准请创建新样本。">
    {canEdit(access)&&<details><summary>新增样本</summary><NewCase access={access} reload={query.reload}/></details>}
    <QueryState {...query} retry={query.reload}/>{query.data?.items.map(item=><p key={item.id}><a href={`#/evaluations/${item.id}`}>{item.name||'正文已清除的样本'}</a> · 保留至 {timestamp(item.expires_at)}</p>)}
    <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
  </Page>;
}

function NewCase({access,reload}:{access:Access;reload:()=>void}) {
  const [name,setName]=useState(''),[input,setInput]=useState(''),[expected,setExpected]=useState(''),[criteria,setCriteria]=useState(''),[consent,setConsent]=useState(false),action=useAction();
  async function create(){
    const item=await apiRequest<Schema['EvaluationCase']>('/evaluations/cases',{method:'POST',csrf:access.session.csrf,body:{project_id:access.project,name,input,expected,criteria:criteria.split('\n').filter(v=>v.trim()),retain_input:consent}});
    if(action.active()){reload();action.navigate(`/evaluations/${item.id}`);}
  }
  return <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(create);}}><fieldset disabled={action.busy}><legend>样本与验收标准</legend>
    <label className="field">样本名称<input required maxLength={128} value={name} onChange={e=>setName(e.target.value)}/></label>
    <label className="field">固定输入<textarea maxLength={16384} rows={6} value={input} onChange={e=>setInput(e.target.value)}/></label>
    <label className="field">预期结果<textarea required maxLength={8192} value={expected} onChange={e=>setExpected(e.target.value)}/></label>
    <label className="field">验收标准（每行一项，最多 20 项）<textarea required value={criteria} onChange={e=>setCriteria(e.target.value)}/></label>
    <label><input type="checkbox" required checked={consent} onChange={e=>setConsent(e.target.checked)}/>同意保留输入、预期和标准 30 天，元数据最多保留 90 天；不包含密钥。</label>
    <button>保存固定样本</button></fieldset><ActionNotice {...action}/></form>;
}

export function CaseDetail({id,access}:{id:string;access:Access}) {
  const query=useQuery<Schema['EvaluationCase']>(`/evaluations/cases/${id}`),item=query.data,action=useAction();
  const [confirm,setConfirm]=useState(false),scoped=item?.project_id===access.project;
  async function retire(){await apiRequest(`/evaluations/cases/${id}`,{method:'DELETE',csrf:access.session.csrf});if(action.active())query.reload();}
  return <Page title="样本版本比较" actions={<a href="#/evaluations">返回样本列表</a>}>
    <QueryState {...query} retry={query.reload}/>{item&&<>
      <h2>{item.name||'样本正文已清除'}</h2><p>保留至 {timestamp(item.expires_at)}</p>
      {!item.payload_expired&&<><h3>固定输入</h3><pre>{item.input}</pre><h3>预期结果</h3><pre>{item.expected}</pre><ul>{item.criteria.map((text,index)=><li key={index}>{text}</li>)}</ul>
        {scoped&&canEdit(access)&&<CaseExecution access={access} item={item}/>}
        {scoped&&(isManager(access)||(canEdit(access)&&item.actor_id===access.session.id))&&<><button disabled={action.busy} onClick={()=>setConfirm(!confirm)}>停用样本</button>{confirm&&<div className="notice">停用会立即清除输入、预期和标准，保留历史关联。<button disabled={action.busy} onClick={()=>void action.run(retire)}>确认停用并清除正文</button></div>}</>}
      </>}<ActionNotice {...action}/><Comparisons id={id}/>
    </>}
  </Page>;
}

function CaseExecution({access,item}:{access:Access;item:Schema['EvaluationCase']}) {
  const [kind,setKind]=useState('agent'),[version,setVersion]=useState<Schema['CatalogVersion']>(),action=useAction(),key=useRef(crypto.randomUUID());
  const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500||action.error.status===409),disabled=action.busy||uncertain;
  async function execute(){
    const accepted=await apiRequest<Schema['Submission']>(`/evaluations/cases/${item.id}/runs`,{method:'POST',csrf:access.session.csrf,key:key.current,body:{version_id:version!.id}});
    action.navigate(`/requests/${receipt(accepted)}`);
  }
  return <form onSubmit={e=>{e.preventDefault();void action.run(execute);}}>
    <label className="field">评测配置类型<select value={kind} disabled={disabled} onChange={e=>{setKind(e.target.value);setVersion(undefined);}}><option value="agent">Agent 角色</option><option value="workflow">工作流</option></select></label>
    <VersionPicker key={kind} kind={kind} value={version?.id??''} change={setVersion} disabled={disabled}/>
    <button disabled={!version||action.busy}>{uncertain?'重试同一样本运行':'用此版本运行固定样本'}</button>
    {uncertain&&<p>结果尚未确认，版本已冻结；重试沿用同一个请求键。</p>}<ActionNotice {...action}/>
  </form>;
}

function Comparisons({id}:{id:string}) {
  const [offset,setOffset]=useState(0),query=useQuery<Schema['EvaluationRunPage']>(`/evaluations/cases/${id}/runs?offset=${offset}`,10000);
  return <section><h2>同一样本的运行记录</h2><QueryState {...query} retry={query.reload}/>
    <table><thead><tr><th>固定版本</th><th>运行</th><th>人工评价</th><th>时间</th></tr></thead><tbody>{query.data?.items.map(item=><tr key={item.request_id}><td>{item.version_id}</td><td><a href={item.run_id?`#/runs/${item.run_id}`:`#/requests/${item.request_id}`}>{item.run_id?'查看运行':'查询回执'}</a></td><td>{decisions[item.decision]??'未评定'}</td><td>{timestamp(item.created_at)}</td></tr>)}</tbody></table>
    <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
  </section>;
}
