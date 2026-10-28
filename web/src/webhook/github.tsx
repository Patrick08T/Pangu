import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {VersionPicker} from '../catalog/selection';
import {ActionNotice,Page,Pager,QueryState,type Access,type Schema} from '../console/components';

export function GithubHooks({access}:{access:Access}) {
 const [offset,setOffset]=useState(0),query=useQuery<Schema['GithubHookPage']>(`/github/hooks?project_id=${access.project}&offset=${offset}`);
 return <Page title="GitHub 任务入口" description="明确启用后，指定仓库的 Issue 标签事件可触发固定版本。不会发送消息或修改 GitHub。">
  <NewHook access={access} reload={query.reload}/><QueryState {...query} retry={query.reload}/>
  {query.data?.items.map(item=><Hook key={item.id} item={item} access={access} reload={query.reload}/>)}
  <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
 </Page>;
}

function NewHook({access,reload}:{access:Access;reload:()=>void}) {
 const [kind,setKind]=useState('workflow'),[version,setVersion]=useState<Schema['CatalogVersion']>(),[repository,setRepository]=useState(''),[label,setLabel]=useState('pangu'),[secret,setSecret]=useState(''),[enabled,setEnabled]=useState(false),action=useAction();
 const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
 async function create(){await apiRequest('/github/hooks',{method:'POST',csrf:access.session.csrf,body:{project_id:access.project,version_id:version!.id,repository,label,secret_ref:secret,enabled}});if(action.active()){setEnabled(false);reload();}}
 return <details><summary>启用一个固定入口</summary><form onSubmit={e=>{e.preventDefault();void action.run(create);}}><fieldset disabled={action.busy||uncertain}><legend>仓库、版本与签名</legend>
  <label className="field">仓库（owner/repo）<input required maxLength={256} value={repository} onChange={e=>setRepository(e.target.value)}/></label>
  <label className="field">触发标签<input required maxLength={64} value={label} onChange={e=>setLabel(e.target.value)}/></label>
  <label className="field">服务端签名密钥引用<input required pattern="PANGU_HOOK_SECRET_[A-Z][A-Z0-9_]{0,63}" value={secret} onChange={e=>setSecret(e.target.value)}/></label>
  <label className="field">版本类型<select value={kind} onChange={e=>{setKind(e.target.value);setVersion(undefined);}}><option value="workflow">工作流</option><option value="agent">Agent</option></select></label>
  <VersionPicker key={kind} kind={kind} value={version?.id??''} change={setVersion}/>
  <p>输入固定为 repository、issue_number、issue_title、issue_body、issue_url 的 JSON。版本须接受此输入；可写 Codex 节点须带审批。Issue 中的 URL 不会自动获得访问授权。</p>
  <label><input type="checkbox" required checked={enabled} onChange={e=>setEnabled(e.target.checked)}/>明确启用此入口，允许符合条件的已签名 Issue 发起运行</label><button disabled={!version||!enabled}>保存并启用</button>
 </fieldset>{uncertain&&<p>创建结果未知，请刷新下方列表核对，不要重复创建。</p>}<button type="button" onClick={reload}>刷新入口列表</button><ActionNotice {...action}/></form></details>;
}

function Hook({item,access,reload}:{item:Schema['GithubHook'];access:Access;reload:()=>void}) {
 const action=useAction(),[confirm,setConfirm]=useState(false),[offset,setOffset]=useState(0),query=useQuery<Schema['GithubEventPage']>(`/github/hooks/${item.id}/events?offset=${offset}`,10000);
 async function disable(){await apiRequest(`/github/hooks/${item.id}`,{method:'DELETE',csrf:access.session.csrf});if(action.active()){setConfirm(false);reload();}}
 return <article><h2>{item.repository} · 标签 {item.label} · {item.enabled?'已启用':'已停用'}</h2><p>固定版本：{item.version_id}</p>
  <p>接收地址：<code>{window.location.origin}/api/hooks/github/{item.id}</code></p><p>GitHub Webhook 选择 JSON 与 Issues 事件。Secret 应与服务端 {item.secret_ref} 一致；本地地址需要维护者配置可达的 HTTPS 反向代理。</p>
  {item.enabled&&<><button disabled={action.busy} onClick={()=>setConfirm(!confirm)}>停用入口</button>{confirm&&<p>停用后不接受新事件，已接受的提交继续执行。<button disabled={action.busy} onClick={()=>void action.run(disable)}>确认停用</button></p>}</>}<ActionNotice {...action}/>
  <QueryState {...query} retry={query.reload}/>{query.data?.items.map(event=><p key={event.delivery_id}>事件 {event.delivery_id} · <a href={event.run_id?`#/runs/${event.run_id}`:`#/requests/${event.request_id}`}>查询运行回执</a></p>)}<Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
 </article>;
}
