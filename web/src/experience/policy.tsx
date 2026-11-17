import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,type Schema} from '../console/components';
import {groups,readDraft,serializeDraft,validateDraft,type Draft,type Field} from './policy-fields';
import {useDraftGuard} from './draft-guard';
export type PolicyProps={policy:Schema['ExperiencePolicy'];project:string;csrf:string;permissions:{manage_policy?:boolean;manage_sources?:boolean};features:{memory_read?:boolean;claim_verification?:boolean;evolution?:boolean;delivery_promotion?:boolean};supportedMetrics?:string[];};

export function PolicyEditor({policy,project,csrf,permissions,features,supportedMetrics=[]}:PolicyProps){
  const [draft,setDraft]=useState(()=>readDraft(policy)),[revision,setRevision]=useState(policy.revision);
  const [conflict,setConflict]=useState(false),[latest,setLatest]=useState<Schema['ExperiencePolicy']>(),action=useAction();
  const [baseline,setBaseline]=useState(()=>JSON.stringify(readDraft(policy)));
  useDraftGuard(JSON.stringify(draft)!==baseline);
  const path=`/projects/${encodeURIComponent(project)}/experience-policy`,allowed=permissions.manage_policy===true&&!!csrf;
  const change=(path:string,value:string)=>setDraft(old=>({...old,values:{...old.values,[path]:value}}));
  async function save(signal:AbortSignal){
    const issue=validateDraft(draft);if(issue)throw new Error(issue);
    if(draft.enabled&&(features.claim_verification!==true||!supportedMetrics.includes(draft.values['evaluation.primary_metric'])))throw new Error('服务未确认所需学习能力或主指标；请保持关闭草稿');
    if(draft.auto_publish&&features.evolution!==true)throw new Error('服务未确认自动发布能力');
    if(draft.read_enabled&&features.memory_read!==true)throw new Error('服务未确认经验读取能力');
    try{
      const saved=await apiRequest<Schema['ExperiencePolicy']>(path,{method:'PUT',csrf,body:{expected_revision:revision,policy:serializeDraft(draft)},signal});
      if(!validPolicyReply(saved)||saved.revision<=revision)throw new Error('服务未确认新修订；本地编辑已保留，请读取最新修订核对。');
      if(action.active()){setRevision(saved.revision);setDraft(readDraft(saved));setBaseline(JSON.stringify(readDraft(saved)));setLatest(undefined);}
    }catch(error){if(error instanceof APIError&&error.status===409){setConflict(true);setLatest(undefined);}throw error;}
  }
  async function loadLatest(signal:AbortSignal){
    const item=await apiRequest<Schema['ExperiencePolicy']>(path,{signal});
    if(!validPolicyReply(item)||item.revision<revision)throw new Error('最新策略回执无效，保留当前编辑');
    if(action.active())setLatest(item);
  }
  return <section><h2>项目学习与复用策略</h2><p>当前编辑基于修订 {revision}。空白门槛不是零容差，未完整登记只能保存关闭的草稿。</p>
    {!allowed&&<p className="notice">服务未授予策略管理权限；仅可查看。</p>}
    <form onSubmit={event=>{event.preventDefault();if(allowed&&!conflict)void action.run(save,'策略已保存；是否可执行仍取决于服务能力与授权');}}>
      <fieldset disabled={!allowed||action.busy}><legend>独立开关与显式授权</legend>
        <PolicySwitches draft={draft} change={setDraft} permissions={permissions} features={features}/>
        {groups.map(group=><details key={group.title} open><summary>{group.title}</summary><div className="experience-fields">{group.fields.map(field=><PolicyField key={field.path} field={field} value={draft.values[field.path]} change={change} promotion={features.delivery_promotion===true} supportedMetrics={supportedMetrics}/>)}</div></details>)}
        <label className="check"><input type="checkbox" checked={draft.source_retention_authorized} disabled={permissions.manage_sources!==true} onChange={event=>setDraft({...draft,source_retention_authorized:event.target.checked})}/>授权在登记保留期内保存脱敏最小来源证据</label>
        <p>保留授权不扩大业务权限。撤销/过期会使衍生经验、成果和审批依赖失效；自动回退不撤销已经发生的业务副作用。</p>
        <button className="primary" disabled={!allowed||action.busy||conflict}>保存策略</button>
      </fieldset>
    </form>
    {conflict&&<div className="notice error" role="alert"><strong>策略已被其他人修改，本地编辑已保留。</strong><p>先读取最新修订并核对差异，不会自动覆盖或重试保存。</p></div>}
    <button disabled={action.busy} onClick={()=>void action.run(loadLatest,'已读取最新修订，未替换本地编辑')}>读取最新修订（保留编辑）</button>
    {latest&&<LatestPolicy latest={latest} draft={draft} allowed={allowed} apply={()=>{setRevision(latest.revision);setConflict(false);setLatest(undefined);}}/>}
    <ActionNotice {...action}/>
  </section>;
}

function validPolicyReply(value:unknown):value is Schema['ExperiencePolicy']{
  if(!value||typeof value!=='object')return false;
  const item=value as Record<string,unknown>;
  return Number.isSafeInteger(item.revision)&&Number(item.revision)>=0
    &&['enabled','read_enabled','auto_publish','revoked'].every(key=>typeof item[key]==='boolean')
    &&['roles','workflows','entrypoints','environments'].every(key=>item[key]===null||(Array.isArray(item[key])&&item[key].every(part=>typeof part==='string')))
    &&['extractor_version','generator_version'].every(key=>typeof item[key]==='string')
    &&['learning_budget','evaluation_budget','evaluation','delivery','observation'].every(key=>item[key]!==null&&typeof item[key]==='object'&&!Array.isArray(item[key]));
}

function PolicySwitches({draft,change,permissions,features}:{draft:Draft;change:(value:Draft)=>void;permissions:PolicyProps['permissions'];features:PolicyProps['features']}){
  return <><label className="check"><input type="checkbox" checked={draft.enabled} disabled={!draft.enabled&&features.claim_verification!==true} onChange={event=>change({...draft,enabled:event.target.checked})}/>自动学习 / 进化总开关</label>
    <p>关闭后停止新学习、晋级与新任务经验水位冻结；已有冻结水位只有在读取开关开启且未撤销时仍可读取。</p>
    <label className="check"><input type="checkbox" checked={draft.read_enabled} disabled={!draft.read_enabled&&features.memory_read!==true} onChange={event=>change({...draft,read_enabled:event.target.checked})}/>允许运行读取经验</label>
    <p>独立控制运行读取；关闭会使已有经验依赖失效，可能暂停成果复用或要求重新审批。</p>
    <label className="check"><input type="checkbox" checked={draft.auto_publish} disabled={!draft.auto_publish&&features.evolution!==true} onChange={event=>change({...draft,auto_publish:event.target.checked})}/>评测通过后自动发布</label>
    {features.evolution!==true&&<p className="notice">自动进化执行能力尚未确认；只能保留关闭草稿，不能以配置保存冒充发布完成。</p>}
    <label className="check"><input type="checkbox" checked={draft.revoked} disabled={draft.revoked||permissions.manage_sources!==true} onChange={event=>change({...draft,revoked:event.target.checked})}/>撤销此策略授权（与关闭学习不同）</label></>;
}

function PolicyField({field,value,change,promotion,supportedMetrics}:{field:Field;value:string;change:(path:string,value:string)=>void;promotion:boolean;supportedMetrics:string[]}){
  return <label className="field">{field.label}{field.kind==='list'?<textarea rows={2} value={value} onChange={event=>change(field.path,event.target.value)}/>:field.kind==='select'?<select value={value} onChange={event=>change(field.path,event.target.value)}><option value="">请显式选择</option>{field.options?.map(([key,label])=><option key={key} value={key} disabled={(field.path==='delivery.mode'&&['canary','serving'].includes(key)&&!promotion)||(field.path==='evaluation.primary_metric'&&!supportedMetrics.includes(key))}>{label}</option>)}</select>:<input type={['number','nullable'].includes(field.kind)?'number':'text'} step="any" value={value} onChange={event=>change(field.path,event.target.value)}/>}</label>;
}

function LatestPolicy({latest,draft,allowed,apply}:{latest:Schema['ExperiencePolicy'];draft:Draft;allowed:boolean;apply:()=>void}){
  const remote=readDraft(latest),fields=groups.flatMap(group=>group.fields).filter(field=>remote.values[field.path]!==draft.values[field.path]);
  return <aside className="panel"><h3>服务器修订：{latest.revision}</h3><p>服务器开关：学习 {remote.enabled?'开':'关'} / 读取 {remote.read_enabled?'开':'关'} / 发布 {remote.auto_publish?'开':'关'} / 授权 {remote.revoked?'已撤销':'未撤销'} / 来源保留 {remote.source_retention_authorized?'已授权':'未授权'}</p>
    <dl>{fields.map(field=><div key={field.path}><dt>{field.label}</dt><dd>服务器：{remote.values[field.path]||'未填写'}；本地：{draft.values[field.path]||'未填写'}</dd></div>)}</dl>
    <p>此操作只更新比较基准，保留所有本地字段；再次保存前请确认不会覆盖他人需要保留的变更。</p><button disabled={!allowed||remote.revoked} onClick={apply}>已核对差异，以最新修订作为保存基准</button>
  </aside>;
}
