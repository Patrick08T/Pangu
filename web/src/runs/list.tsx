import { useState } from 'react';
import { useQuery } from '../api/hooks';
import { Page,Pager,QueryState,Status,timestamp,canEdit,type Access,type Schema } from '../console/components';

export function Runs({ access }: {access:Access}) {
  const [offset,setOffset]=useState(0),[status,setStatus]=useState(''),[since,setSince]=useState('');
  const [window,setWindow]=useState<{since:string;until:string}>();
  const params=new URLSearchParams({project_id:access.project,offset:String(offset),limit:'20'});
  if(status)params.set('status',status);
  if(since)params.set('since',new Date(since).toISOString());
  if(offset>0&&window){params.set('since',window.since);params.set('until',window.until);}
  const query=useQuery<Schema['RunList']>(access.project?`/runs?${params}`:null,offset===0?5000:0);
  function changePage(next:number){if(query.data)setWindow({since:query.data.since,until:query.data.until});setOffset(next);}
  return <Page title="任务记录" description="查看任务进度和执行结果，状态会自动更新。" actions={canEdit(access)&&<a className="button primary" href="#/onboarding">新建任务</a>}>
    <form className="toolbar" onSubmit={event=>event.preventDefault()}>
      <label>状态<select value={status} onChange={e=>{setStatus(e.target.value);setOffset(0);}}><option value="">全部状态</option>{[['queued','排队中'],['running','执行中'],['cancelling','取消中'],['succeeded','已完成'],['failed','失败'],['cancelled','已取消']].map(([value,label])=><option value={value} key={value}>{label}</option>)}</select></label>
      <label>起始时间<input type="datetime-local" value={since} onChange={e=>{setSince(e.target.value);setOffset(0);}}/></label>
      <button type="button" onClick={query.reload}>刷新</button>
    </form>
    <QueryState {...query} retry={query.reload} empty={!query.data?.items.length?'当前页没有符合条件的运行。可更改筛选或继续翻页。':undefined}/>
    {!!query.data?.items.length&&<div className="table-scroll"><table><thead><tr><th>运行</th><th>状态</th><th>创建时间</th><th>开始时间</th><th>结束时间</th></tr></thead><tbody>{query.data.items.map(item=><tr key={item.id}>
      <td><a href={`#/runs/${item.id}`}>{item.title||`任务 · ${timestamp(item.created_at)}`}</a><small className="identifier">{item.id.slice(0,8)}</small></td><td><Status value={item.status}/></td><td>{timestamp(item.created_at)}</td><td>{timestamp(item.started_at)}</td><td>{timestamp(item.finished_at)}</td>
    </tr>)}</tbody></table></div>}
    {query.data&&<Pager offset={offset} next={query.data.next_offset} change={changePage}/>}
  </Page>;
}
