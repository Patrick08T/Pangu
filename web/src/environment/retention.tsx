import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Pager,QueryState,timestamp,type Access,type Schema} from '../console/components';

const labels:Record<string,string>={protected:'受保护：活动状态或保留期尚未确认',eligible:'可清理',purged:'已清理',leased:'正在读取或下载，稍后重试',failed:'本项失败，可重试'};
export function Retention({access}:{access:Access}) {
 const query=useQuery<Schema['RetentionPage']>('/retention'),action=useAction(),[offset,setOffset]=useState(0);
 const history=useQuery<Schema['RetentionHistoryPage']>(`/retention/history?offset=${offset}&limit=20`);
 const [result,setResult]=useState<Schema['RetentionPage']>(),[confirmed,confirm]=useState(false);
 async function clean(){const page=await apiRequest<Schema['RetentionPage']>('/retention',{method:'POST',csrf:access.session.csrf});if(action.active()){setResult(page);confirm(false);query.reload();setOffset(0);history.reload();}}
 return <section><h2>数据保留与清理</h2><p>运行与幂等记录 90 天；日志、产物、工作区和重跑基准 30 天；审计 180 天。只清理当前 Worker 的托管存储，活动运行及下载租约受保护，配置版本保留。</p>
  <QueryState {...query} retry={query.reload}/><ActionNotice {...action}/>
  <p>以下最多 100 条候选按最久未检查排序，执行时会重新核查条件。</p>
  <div className="table-scroll"><table><thead><tr><th>运行</th><th>创建时间</th><th>本次状态</th></tr></thead><tbody>{(result??query.data)?.items.map(item=><tr key={item.run_id}><td>{item.run_id}</td><td>{timestamp(item.created_at)}</td><td>{labels[item.status]}</td></tr>)}</tbody></table></div>
  <label className="checkbox"><input type="checkbox" checked={confirmed} onChange={event=>confirm(event.target.checked)}/>确认删除到期产物，删除后无法从 Pangu 恢复或重跑</label>
  <button disabled={!confirmed||action.busy||!query.data} onClick={()=>void action.run(clean,'本批清理已检查，详见逐项结果；结果未知时可安全重试。')}>执行一批清理</button>
  <h3>清理执行记录</h3><p>清理记录独立保留 180 天，运行历史删除后仍可追溯。</p>
  <QueryState {...history} retry={history.reload} empty={history.data&&!history.data.items.length?'尚无清理记录。':undefined}/>
  <div className="table-scroll"><table><thead><tr><th>运行</th><th>Worker</th><th>执行时间</th><th>结果</th></tr></thead><tbody>{history.data?.items.map(item=><tr key={item.id}><td>{item.run_id}</td><td>{item.worker_id}</td><td>{timestamp(item.checked_at)}</td><td>{labels[item.status]}</td></tr>)}</tbody></table></div>
  {history.data&&<Pager offset={offset} next={history.data.next_offset} change={setOffset}/>}
 </section>;
}
