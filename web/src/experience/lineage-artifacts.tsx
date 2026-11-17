import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';
import {AuditStatus,auditVisible} from './audit';

type Artifact=Schema['ExperienceArtifactSummary'];
export function References({items}:{items:Schema['ExperienceReference'][]}){
  return items.length>0?<section><h5>冻结经验引用</h5><ul>{items.map((ref,index)=><li key={`${ref.version_id}:${index}`}><span>版本：{ref.version_id} · 投放修订 {ref.delivery_revision}</span><div className="identifier">{ref.body_hash}</div></li>)}</ul></section>:null;
}

export function ArtifactCard({project,item,ancestry}:{project:string;item:Artifact;ancestry:string[]}){
  return <article className="panel"><h5>成果：{item.id}</h5><p>节点：{item.node_id} · 类型：{item.kind} · generation {item.generation}</p><AuditStatus state={item}/>
    {auditVisible(item)&&<><dl>{[['产物哈希',item.output_hash],['输入哈希',item.input_hash],['实际执行输入哈希',item.execution_input_hash]].map(([name,value])=><div key={name}><dt>{name}</dt><dd className="identifier">{value||'未登记'}</dd></div>)}</dl>
      <References items={item.references??[]}/>{item.parents?.map((parent,index)=>ancestry.includes(parent.id)?<p key={`${parent.id}:${index}`} className="notice error">父引用形成环，不能视为有效影响链。</p>:<ParentArtifact key={`${parent.id}:${index}`} project={project} parent={parent} ancestry={ancestry}/>)}</>}
  </article>;
}

function ParentArtifact({project,parent,ancestry}:{project:string;parent:Schema['ExperienceArtifactReference'];ancestry:string[]}){
  const [open,setOpen]=useState(false);
  const path=`/projects/${encodeURIComponent(project)}/experience-artifacts/${encodeURIComponent(parent.id)}`;
  const query=useQuery<Artifact>(open?path:null),item=!query.loading&&!query.error?query.data:undefined;
  const visible=item&&auditVisible(item);
  const same=item?.id===parent.id&&(!visible||item.output_hash===parent.hash);
  return <section><p>父成果引用：{parent.id}</p><button aria-expanded={open} onClick={()=>setOpen(!open)}>{open?'收起父成果':'查看父成果'} {parent.id}</button>
    {open&&<><QueryState {...query} retry={query.reload}/>{item&&(!same?<p role="alert">父成果身份或哈希不一致，已隐藏详情。</p>:<ArtifactCard item={item} project={project} ancestry={[...ancestry,parent.id]}/>)}</>}
  </section>;
}
