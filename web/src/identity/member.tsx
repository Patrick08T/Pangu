import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,type Access,type Schema} from '../console/components';

export function Members({access}:{access:Access}) {
  const query=useQuery<Schema['Member'][]>(`/projects/${access.project}/members`),action=useAction();
  const [openID,setOpenID]=useState(''),[role,setRole]=useState('viewer');
  const [confirm,setConfirm]=useState<{member:Schema['Member'];operation:string}>();
  async function grant(){await apiRequest(`/projects/${access.project}/members`,{method:'PUT',csrf:access.session.csrf,body:{open_id:openID,role}});setOpenID('');query.reload();}
  async function change(){
    if(!confirm)return;
    const {member,operation}=confirm;
    await apiRequest(operation==='remove'?`/projects/${access.project}/members/${member.id}`:`/users/${member.id}/${operation}`,{method:operation==='remove'?'DELETE':'POST',csrf:access.session.csrf});setConfirm(undefined);query.reload();
  }
  const operation=confirm?.operation==='remove'?'移除项目成员':confirm?.operation==='enable'?'恢复账号':'全局禁用';
  return <section className="panel"><h2>飞书成员与项目角色</h2>
    <p className="muted">按当前飞书应用的 Open ID 邀请或修改角色，不读取完整通讯录。降为查看者会暂停该成员在此项目的未来计划。</p>
    <QueryState {...query} retry={query.reload} empty={!query.data?.length?'尚无已登记成员。':undefined}/>
    {!!query.data?.length&&<div className="table-scroll"><table><thead><tr><th>成员</th><th>角色 / 状态</th><th>操作</th></tr></thead><tbody>{query.data.map(member=><tr key={member.id}><td>{member.name||'待首次登录'}<small className="identifier">{member.open_id}</small></td><td>{member.role} · {member.disabled?'账号已禁用':member.enabled?'已授权':'已移除'}</td><td><div className="toolbar"><button onClick={()=>{setOpenID(member.open_id);setRole(member.role);}}>编辑角色</button>{member.enabled&&<button onClick={()=>setConfirm({member,operation:'remove'})}>移除项目成员</button>}{access.session.admin&&<button onClick={()=>setConfirm({member,operation:member.disabled?'enable':'disable'})}>{member.disabled?'恢复账号':'全局禁用'}</button>}</div></td></tr>)}</tbody></table></div>}
    {confirm&&<div className="notice" role="alert"><p>{operation}：{confirm.member.name||confirm.member.open_id}。</p><p>{confirm.operation==='enable'?'恢复账号不会恢复旧会话、旧 Token 或已暂停计划。':confirm.operation==='remove'?'撤销本项目权限和 Token，未来计划暂停；已接受的运行继续。':'所有会话和 Token 立即失效，所有项目的未来计划暂停；已接受的运行继续。'}</p><button disabled={action.busy} onClick={()=>void action.run(change,'成员状态已更新')}>确认{operation}</button><button onClick={()=>setConfirm(undefined)}>返回</button></div>}
    <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(grant,'成员权限已保存');}}><fieldset disabled={action.busy}><legend>登记 / 修改项目成员</legend>
      <label className="field">飞书 Open ID<input required value={openID} onChange={e=>setOpenID(e.target.value)}/></label>
      <label className="field">项目角色<select value={role} onChange={e=>setRole(e.target.value)}><option value="viewer">查看者</option><option value="operator">操作者</option><option value="manager">管理者</option></select></label><button className="primary">保存成员权限</button>
    </fieldset></form><ActionNotice {...action}/>
  </section>;
}
