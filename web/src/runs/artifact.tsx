import { useState } from 'react';
import { useQuery } from '../api/hooks';
import { QueryState,type Schema } from '../console/components';
import {Download} from '../api/download';

export function Artifacts({ run }: { run:string }) {
  const query=useQuery<Schema['ArtifactManifest']>(`/runs/${run}/artifacts`,5000),[selected,select]=useState<string>();
  const files=query.data?.files??[],current=files.find(file=>file.id===selected)??files.find(file=>file.path.startsWith('.pangu/reports/')&&!file.blocked)??files.find(file=>!file.blocked);
  return <section><div className="section-heading"><h2>任务结果</h2>{files.length>0&&<Download path={`/runs/${run}/artifacts.zip`} filename={`${run}.zip`}>下载全部结果</Download>}</div>
    <p className="muted">已完成的报告和文件会显示在这里。文件修改保存在独立工作区，不会自动覆盖源项目。</p>
    <QueryState {...query} retry={query.reload} empty={query.data&&!query.data.files.length?'尚无文件变更或报告。':undefined}/>
    <div className="artifact-layout"><ul className="file-list">{query.data?.files.map(file=><li key={file.id}>
      <button aria-pressed={current?.id===file.id} onClick={()=>select(file.id)}><span className="change-kind">{file.change}</span><span>{file.path.startsWith('.pangu/reports/')?'产品报告':file.path}</span><small>{file.size.toLocaleString()} 字节{file.binary?' · 二进制':''}{file.blocked?' · 限制访问':''}</small></button>
    </li>)}</ul>{current&&<ArtifactDetail key={current.id} run={run} id={current.id}/>}</div>
  </section>;
}

function ArtifactDetail({run,id}:{run:string;id:string}) {
  const query=useQuery<{file:Schema['ArtifactFile'];diff:string;truncated:boolean}>(`/runs/${run}/artifacts/${id}/diff`);
  const report=query.data?.file.path.startsWith('.pangu/reports/');
  return <div className="artifact-detail"><QueryState {...query} retry={query.reload}/>{query.data&&<>
    <div className="section-heading"><h3>{report?'报告正文':query.data.file.path}</h3>{query.data.file.change!=='D'&&!query.data.file.blocked&&<Download path={`/runs/${run}/artifacts/${id}`} filename={query.data.file.path.split('/').pop()??id}>{report?'下载报告':'下载文件'}</Download>}</div>
    {query.data.file.binary?<p>二进制文件不显示文本差异，可下载后检查。</p>:<pre className={report?'report-body':'diff'} tabIndex={0}>{query.data.diff||'没有可显示的文本差异。'}</pre>}
    {query.data.truncated&&<p className="notice">差异已截断，仅显示前 1 MiB；下载完整产物进行检查。</p>}
  </>}</div>;
}
