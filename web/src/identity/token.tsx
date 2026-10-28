import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,QueryState,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';

const labels:Record<string,string>={read:'读取',run:'发起运行',configure:'修改配置',schedule:'创建计划',manage_run:'管理运行',manage_schedule:'管理计划',manage_connections:'管理模型连接',manage_factory:'管理工厂对话'};
export function Tokens({access}:{access:Access}) {
  const query=useQuery<Schema['APIToken'][]>('/auth/tokens'),action=useAction();
  const [name,setName]=useState(''),[days,setDays]=useState(30),[scopes,setScopes]=useState<string[]>(['read']),[value,setValue]=useState('');
  const allowed=Object.keys(labels).filter(scope=>scope==='read'||(scope==='manage_connections'?isManager(access):canEdit(access)));
  async function create(){
    const token=await apiRequest<Schema['TokenIssued']>('/auth/tokens',{method:'POST',csrf:access.session.csrf,body:{project_id:access.project,name,scopes,expires_at:new Date(Date.now()+days*86400000).toISOString()}});
    setValue(token.value);query.reload();
  }
  async function revoke(id:string){await apiRequest(`/auth/tokens/${id}`,{method:'DELETE',csrf:access.session.csrf});setValue('');query.reload();}
  return <Page title="我的 API Token" description="每个 Token 仅限一个项目，权限同时受当前成员角色限制；请勿放入仓库或聊天。">
    <QueryState {...query} retry={query.reload} empty={!query.data?.length?'尚未创建 Token。':undefined}/>
    {!!query.data?.length&&<div className="table-scroll"><table><thead><tr><th>名称 / 项目</th><th>权限</th><th>到期 / 状态</th><th>操作</th></tr></thead><tbody>{query.data.map(token=><tr key={token.id}><td>{token.name}<small className="identifier">{token.project_id}</small></td><td>{token.scopes.map(scope=>labels[scope]).join('、')}</td><td>{timestamp(token.expires_at)}<small>{token.revoked_at?'已撤销':new Date(token.expires_at)<new Date()?'已过期':'有效'}</small></td><td><button disabled={!!token.revoked_at||action.busy} onClick={()=>void action.run(()=>revoke(token.id),'Token 已撤销')}>撤销 Token</button></td></tr>)}</tbody></table></div>}
    {value&&<div className="notice"><strong>Token 仅在此时返回一次，离开页面后无法再次查看。</strong><label className="field">新 Token<input readOnly value={value} autoComplete="off"/></label><button onClick={()=>setValue('')}>我已保存，隐藏 Token</button></div>}
    {access.project&&<form className="editor" onSubmit={e=>{e.preventDefault();void action.run(create,'Token 已创建');}}><fieldset disabled={action.busy||!!value}><legend>为当前项目创建 Token</legend>
      <label className="field">用途名称<input required maxLength={128} value={name} onChange={e=>setName(e.target.value)}/></label>
      <label className="field">有效天数（1–90）<input type="number" min={1} max={90} required value={days} onChange={e=>setDays(Number(e.target.value))}/></label>
      <div className="checks">{allowed.map(scope=><label className="check" key={scope}><input type="checkbox" checked={scopes.includes(scope)} onChange={e=>setScopes(e.target.checked?[...scopes,scope]:scopes.filter(item=>item!==scope))}/>{labels[scope]}</label>)}</div><button className="primary" disabled={!scopes.length}>创建 Token</button>
    </fieldset></form>}<ActionNotice {...action}/>
  </Page>;
}
