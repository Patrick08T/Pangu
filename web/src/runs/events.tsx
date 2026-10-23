import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,timestamp,type Schema} from '../console/components';

export function Events({run,nodes}:{run:string;nodes:Schema['NodeRun'][]}) {
 const [node,setNode]=useState(''),[offset,setOffset]=useState(0);
 const query=useQuery<Schema['EventPage']>(`/runs/${run}/events?offset=${offset}&limit=20${node?`&node_run_id=${encodeURIComponent(node)}`:''}`,offset===0?5000:undefined);
 return <section><h2>调度事件</h2><label>事件范围<select value={node} onChange={event=>{setNode(event.target.value);setOffset(0);}}><option value="">整次运行</option>{nodes.map(item=><option key={item.id} value={item.id}>{item.node_id}</option>)}</select></label>
  <QueryState {...query} retry={query.reload} empty={query.data&&!query.data.items.length?'尚无调度事件。':undefined}/>
  <ul>{query.data?.items.map(event=><li key={event.id}>{timestamp(event.time)} · {event.type} · 第 {event.attempt} 次尝试<pre>{event.message}</pre>{event.truncated&&<p>事件内容已截断。</p>}</li>)}</ul>
  {query.data&&<Pager offset={offset} next={query.data.next_offset} change={setOffset}/>}
 </section>;
}
