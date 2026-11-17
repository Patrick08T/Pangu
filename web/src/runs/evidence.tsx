import {useQuery} from '../api/hooks';
import {QueryState,timestamp,type Schema} from '../console/components';

export function PlatformEvidence({run}:{run:string}){
  const query=useQuery<Schema['EvidenceManifest']>(`/runs/${run}/evidence`);
  return <section><h2>平台原始证据</h2><QueryState {...query} retry={query.reload}/>{query.data&&<>
    <p>采集时间：{timestamp(query.data.collected_at)}。按需打开原始记录核对，来源运行的记录保留原归属。</p>
    {query.data.limitations.map(text=><p className="muted" key={text}>{text}</p>)}
    {!!query.data.missing_sources.length&&<p className="notice">以下来源记录已不可读取：{query.data.missing_sources.join('、')}</p>}
    <div className="table-scroll"><table><thead><tr><th>材料</th><th>归属</th><th>大小</th><th>内容哈希</th></tr></thead>
      <tbody>{query.data.files.map(file=><tr key={file.path}><td><a href={`/api/runs/${run}/evidence?path=${encodeURIComponent(file.path)}`} target="_blank" rel="noreferrer">{file.title}</a></td><td>{file.run_id===run?'当前运行':<a href={`#/runs/${file.run_id}`}>来源运行</a>}</td><td>{file.bytes} 字节</td><td className="identifier">{file.sha256}</td></tr>)}</tbody>
    </table></div>
  </>}</section>;
}
