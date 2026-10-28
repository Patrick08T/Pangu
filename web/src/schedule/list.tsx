import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,Pager,QueryState,Status,canEdit,timestamp,type Access,type Schema} from '../console/components';
import {ScheduleEditor} from './editor';

export function Schedules({access}:{access:Access}) {
  const [offset,setOffset]=useState(0),query=useQuery<Schema['CronPage']>(`/crons?project_id=${access.project}&offset=${offset}`,5000);
  return <Page title="自动化" description="在画布上编排助手，或为已发布流程设置定时任务。定时按 UTC 触发，漏跑不补。" actions={<div className="toolbar"><a className="button" href="#/workflow">可视化编排</a>{canEdit(access)&&<a className="button primary" href="#/schedule/new">新建定时计划</a>}</div>}>
    <QueryState {...query} retry={query.reload} empty={!query.data?.items.length?'当前页没有计划。':undefined}/>
    {!!query.data?.items.length&&<div className="table-scroll"><table><thead><tr><th>名称</th><th>UTC 表达式</th><th>状态</th><th>版本</th></tr></thead><tbody>{query.data.items.map(plan=><tr key={plan.id}><td><a href={`#/schedule/${plan.id}`}>{plan.name}</a></td><td><code>{plan.expression}</code></td><td>{plan.deleted?'已删除':plan.paused?'已暂停':plan.ready?'运行中':'同步中'}</td><td className="identifier">{plan.workflow.version_id??'JSON 快照'}</td></tr>)}</tbody></table></div>}
    <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
  </Page>;
}

function Fires({id}:{id:string}) {
  const [offset,setOffset]=useState(0),query=useQuery<Schema['ScheduleFire'][]>(`/crons/${id}/fires?offset=${offset}`,5000);
  return <section className="panel"><h2>触发记录</h2><QueryState {...query} retry={query.reload} empty={!query.data?.length?'当前页没有触发记录。':undefined}/>
    {!!query.data?.length&&<div className="table-scroll"><table><thead><tr><th>触发时间（UTC）</th><th>结果</th><th>原因 / 请求</th></tr></thead><tbody>{query.data.map(fire=><tr key={fire.slot}><td>{new Date(fire.slot).toISOString()}</td><td><Status value={fire.status}/></td><td>{fire.reason}{fire.request_id&&<a className="identifier" href={`#/requests/${fire.request_id}`}>查询请求 {fire.request_id}</a>}</td></tr>)}</tbody></table></div>}
    <Pager offset={offset} next={query.data?.length===100?offset+100:undefined} change={setOffset}/>
  </section>;
}

export function ScheduleDetail({access,id}:{access:Access;id:string}) {
  const query=useQuery<Schema['Schedule']>(`/crons/${id}`),action=useAction(),[confirm,setConfirm]=useState(false),plan=query.data;
  const editable=plan&&access.project===plan.workflow.project_id&&canEdit(access)&&(access.role==='manager'||plan.created_by===access.session.id)&&!plan.deleted;
  async function remove(){await apiRequest(`/crons/${id}`,{method:'DELETE',csrf:access.session.csrf});setConfirm(false);query.reload();}
  return <Page title={plan?.name??'计划详情'} description="编辑不会覆盖已经接受的运行，版本升级需要明确保存。">
    <QueryState {...query} retry={query.reload}/>
    {plan&&<><p className="identifier">{plan.id} · 创建于 {timestamp(plan.created_at)} · {plan.deleted?'已删除':plan.paused?'已暂停':plan.ready?'已同步':'等待调度同步'}</p>
      {editable?<><ScheduleEditor key={`${plan.id}:${plan.revision}`} access={access} plan={plan} saved={query.reload}/><div className="toolbar">{confirm?<><p>删除后停止未来触发，已接受的运行继续执行。</p><button disabled={action.busy} onClick={()=>void action.run(remove,'计划已删除')}>确认删除计划</button><button onClick={()=>setConfirm(false)}>保留计划</button></>:<button onClick={()=>setConfirm(true)}>删除计划</button>}</div><ActionNotice {...action}/></>:<pre>{JSON.stringify(plan.workflow,null,2)}</pre>}
      <Fires id={id}/></>}
  </Page>;
}

export function NewSchedule({access,version}:{access:Access;version?:string}) {
  return <Page title="新建定时计划">{canEdit(access)?<ScheduleEditor access={access} version={version} saved={()=>{window.location.hash='/schedules';}}/>:<p>当前角色无创建计划权限。</p>}</Page>;
}
