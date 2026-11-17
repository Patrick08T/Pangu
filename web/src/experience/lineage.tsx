import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState,timestamp,type Schema} from '../console/components';
import {AuditStatus,auditVisible,matchReason} from './audit';
import {ArtifactCard,References} from './lineage-artifacts';

type Lineage=Schema['RunExperienceLineage'];
export function RunLineage({project,run}:{project:string;run:string}){
  const [usageCursor,setUsageCursor]=useState(''),[artifactCursor,setArtifactCursor]=useState('');
  const params=new URLSearchParams({limit:'20'});
  if(usageCursor)params.set('usage_cursor',usageCursor);
  if(artifactCursor)params.set('artifact_cursor',artifactCursor);
  const query=useQuery<Lineage>(`/runs/${encodeURIComponent(run)}/experience?${params}`);
  const data=!query.loading&&!query.error?query.data:undefined;
  const valid=data&&completeLineage(data);
  return <section><h3>冻结水位与运行影响链</h3><QueryState {...query} retry={query.reload}/>
    {data&&data.actual_run_id!==run?<p role="alert">返回的运行不匹配，已隐藏影响链。</p>:data&&!valid?<p role="alert">影响链回执不完整，不能视为没有使用经验。</p>:data&&valid&&<>
      <Watermark value={data.watermark}/>
      <p className="notice">{data.chain_complete===true?'服务报告当前已登记影响链完整；不代表可以立即复用，仍须核对实际执行输入、当前授权和审批。':'当前响应未证明影响链完整；分页和未展开父依赖都不能当作完整链。'}</p>
      <h4>节点使用记录</h4><p>使用清单的组合结果不归因于其中任何单条经验。</p>
      {data.usages.items.length===0&&<p>本页没有使用记录；不代表历史上没有使用经验。</p>}
      {data.usages.items.map((item,index)=><Usage key={`${item.node_id}:${item.iteration}:${item.approval_generation}:${index}`} item={item}/>)}
      <CursorPager label="使用记录" page={data.usages} cursor={usageCursor} change={setUsageCursor}/>
      <h4>成果与复用依赖</h4>{data.artifacts.items.length===0&&<p>本页没有已登记成果；不据此声称没有衍生影响。</p>}
      {data.artifacts.items.map(item=><ArtifactCard key={item.id} project={project} item={item} ancestry={[item.id]}/>)}
      <CursorPager label="成果依赖" page={data.artifacts} cursor={artifactCursor} change={setArtifactCursor}/>
    </>}
    <button disabled={query.loading} onClick={query.reload}>刷新运行影响链</button>
  </section>;
}

function completeLineage(value:Lineage){
  const watermark=value.watermark;
  return !!watermark&&typeof watermark.present==='boolean'&&typeof watermark.status==='string'
    &&typeof watermark.proven==='boolean'&&typeof watermark.redacted==='boolean'
    &&Number.isSafeInteger(watermark.generation)&&Number.isSafeInteger(watermark.policy_revision)
    &&(watermark.references===null||Array.isArray(watermark.references))
    &&Array.isArray(value.usages?.items)&&typeof value.usages.has_more==='boolean'
    &&Array.isArray(value.artifacts?.items)&&typeof value.artifacts.has_more==='boolean'
    &&typeof value.chain_complete==='boolean';
}

function Watermark({value}:{value:Schema['ExperienceWatermarkSummary']}){
  return <article className="panel"><h4>任务接收水位</h4><AuditStatus state={value}/>
    {!value.present?<p>未登记经验水位，无法证明这是未使用经验的任务。</p>:<p>水位 generation {value.generation} · 策略修订 {value.policy_revision}</p>}
    {value.present&&auditVisible(value)&&<>{value.hash&&<dl><dt>水位哈希</dt><dd className="identifier">{value.hash}</dd></dl>}
      {value.status==='valid'&&value.proven===true&&(value.references??[]).length===0&&<p>已冻结空经验清单；不同于水位缺失。</p>}<References items={value.references??[]}/></>}
  </article>;
}

function Usage({item}:{item:Schema['ExperienceRunUsageSummary']}){
  return <article className="panel"><h4>节点：{item.node_id} · 循环轮次 {item.iteration}</h4><AuditStatus state={item}/>
    <p>审批 generation {item.approval_generation} · 审批修订 {item.approval_revision}</p>{item.created_at&&<p>登记时间：{timestamp(item.created_at)}</p>}
    {auditVisible(item)&&<><dl><dt>匹配依据</dt><dd>{matchReason(item.match_reason)}</dd><dt>实际注入哈希</dt><dd className="identifier">{item.injection_hash||'未登记，不推断已注入'}</dd></dl><References items={item.references??[]}/></>}
  </article>;
}

function CursorPager({label,page,cursor,change}:{label:string;page:{total?:number;has_more?:boolean;next_cursor?:string};cursor:string;change:(cursor:string)=>void}){
  return <div className="toolbar"><span>当前可见总数：{page.total??'未知'}</span><button disabled={!cursor} onClick={()=>change('')}>{label}返回首页</button><button disabled={page.has_more!==true||!page.next_cursor||page.next_cursor===cursor} onClick={()=>{if(page.next_cursor)change(page.next_cursor);}}>{label}下一页</button>{page.has_more&&!page.next_cursor&&<span>后续游标缺失，不能视为已读完整。</span>}</div>;
}
