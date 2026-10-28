import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';

export function ReportQuality({run}:{run:string}) {
  const query=useQuery<Schema['ReportDetail'][]>(`/runs/${run}/reports`);
  return <section aria-label="交付检查"><QueryState {...query} retry={query.reload}/>{query.data?.map(report=>{
    const data=report.data;
    if(!data)return <p className="notice" key={report.id}>报告缺少结构化证据，尚不能判断是否完成调研。</p>;
    const sources=(data.sources??[]).filter(source=>source.read&&source.id!=='input');
    const facts=(data.claims??[]).filter(claim=>claim.kind==='fact'&&claim.evidence?.some(item=>sources.some(source=>source.id===item.source_id)));
    const research=['research','competition'].includes(data.mode);
    return <div key={report.id} className={`notice ${research&&!facts.length?'error':''}`} role={research&&!facts.length?'alert':'status'}>
      <strong>{!facts.length?(research?'未达到调研验收':sources.length?'证据不足的草案':'仅基于输入的草案'):'资料读取检查'}</strong>
      <p>{!facts.length?'报告没有已读取外部或项目资料支持的事实，不能作为市场结论或项目现状评估。':`实际读取 ${sources.length} 份资料、登记 ${facts.length} 项事实主张；引文是否支持结论仍需核对。`}</p>
      {research&&!facts.length&&<a href="#/onboarding">补充公开搜索词、网址或项目文件后新建任务</a>}
    </div>;
  })}</section>;
}
