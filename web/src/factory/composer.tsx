import {useEffect,useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,type Access,type Schema} from '../console/components';

type Props={access:Access;mode:Schema['FactoryInput']['mode'];setMode:(v:Schema['FactoryInput']['mode'])=>void;message:string;setMessage:(v:string)=>void;connection:string;setConnection:(v:string)=>void;target?:Schema['CatalogObject'];setTarget:(v:Schema['CatalogObject']|undefined)=>void;source:string;setSource:(v:string)=>void;disabled:boolean;busy:boolean;uncertain:boolean;send:()=>void};
export function Composer(p:Props) {
 const connections=useQuery<Schema['ConnectionChoicePage']>('/connections/choices');
 const [offset,setOffset]=useState(0);
 const objects=useQuery<Schema['CatalogPage']>(p.mode!=='build'?`/catalog?offset=${offset}`:null);
 const runs=useQuery<Schema['RunList']>(p.mode==='diagnose'||p.mode==='optimize'?`/runs?project_id=${p.access.project}&limit=100`:null);
 useEffect(()=>{if(!p.connection&&connections.data?.items[0])p.setConnection(connections.data.items[0].id);},[connections.data,p.connection,p.setConnection]);
 const invalid=!p.connection||!p.message.trim()||(['modify','optimize'].includes(p.mode)&&!p.target)||(['diagnose','optimize'].includes(p.mode)&&!p.source);
 return <form className="factory-composer" onSubmit={e=>{e.preventDefault();p.send();}}><fieldset disabled={p.disabled}><legend>继续对话</legend><div className="factory-options">
  <label className="field">工作方式<select value={p.mode} onChange={e=>{p.setMode(e.target.value as Props['mode']);p.setTarget(undefined);p.setSource('');}}><option value="build">创建助手与流程</option><option value="modify">修改现有配置</option><option value="diagnose">诊断任务</option><option value="optimize">基于证据优化</option></select></label>
  <label className="field">生成方案使用的模型<select value={p.connection} onChange={e=>p.setConnection(e.target.value)}><option value="">选择模型连接</option>{connections.data?.items.map(c=><option key={c.id} value={c.id}>{c.name}{c.model?` · ${c.model}`:''}</option>)}</select></label></div>
  <QueryState {...connections} retry={connections.reload}/>{connections.data?.items.length===0&&<p className="notice">尚无共享模型连接。请在<a href="#/connection">模型连接</a>中配置并发布。</p>}
  {p.mode!=='build'&&<><QueryState {...objects} retry={objects.reload}/><label className="field">要修改的助手或流程<select value={p.target?.id??''} onChange={e=>p.setTarget(objects.data?.items.find(o=>o.id===e.target.value))}><option value="">{['modify','optimize'].includes(p.mode)?'请选择目标':'不修改现有配置，只分析或生成新方案'}</option>{objects.data?.items.filter(o=>!o.archived&&['agent','workflow'].includes(o.kind)).map(o=><option key={o.id} value={o.id}>{o.name} · 修订 {o.revision}</option>)}</select></label>{(offset>0||objects.data?.next_offset!==undefined)&&<Pager offset={offset} next={objects.data?.next_offset} change={setOffset}/>}</>}
  {['diagnose','optimize'].includes(p.mode)&&<><QueryState {...runs} retry={runs.reload}/><label className="field">用于分析的任务<select value={p.source} onChange={e=>p.setSource(e.target.value)}><option value="">选择已结束的任务</option>{runs.data?.items.filter(r=>['succeeded','failed','cancelled'].includes(r.status)).map(r=><option key={r.id} value={r.id}>{r.title||r.id} · {r.status}</option>)}</select></label><small>使用该任务的节点结果与错误作为证据；生成候选不代表已经验证优化效果。</small></>}
  <label className="field">告诉工厂助手你的需求<textarea rows={4} maxLength={8000} value={p.message} placeholder="你想完成什么？有哪些资料？希望交付什么结果？" onChange={e=>p.setMessage(e.target.value)}/></label>
 </fieldset><div className="factory-send"><small>每次发送最多调用模型 1 次；生成过程不操作项目文件。</small><button className="primary" disabled={p.busy||(!p.uncertain&&(p.disabled||invalid))}>{p.busy?'正在处理…':p.uncertain?'重试同一条消息':'发送需求'}</button></div></form>;
}
