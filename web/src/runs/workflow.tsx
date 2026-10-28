import { ErrorNotice, Status, type Schema } from '../console/components';

type Dependency = { id: string; depends_on?: string[] };
export function dagLayers<T extends Dependency>(nodes: T[]): T[][] {
  const ids=new Set(nodes.map(node=>node.id));
  if(ids.size!==nodes.length || nodes.length>32)throw Error('节点数量无效或标识重复');
  if(nodes.some(node=>node.depends_on?.some(id=>!ids.has(id))))throw Error('依赖节点不存在');
  const done=new Set<string>(),layers:T[][]=[];
  while(done.size<nodes.length){
    const layer=nodes.filter(node=>!done.has(node.id)&&(node.depends_on??[]).every(id=>done.has(id)));
    if(!layer.length)throw Error('节点依赖存在循环');
    layers.push(layer);layer.forEach(node=>done.add(node.id));
  }
  return layers;
}

export function WorkflowGraph({ definition, nodes }: { definition: Schema['Workflow']; nodes: Schema['NodeRun'][] }) {
  let layers: Schema['Node'][][];
  try { layers=dagLayers(definition.nodes); } catch(error) {return <ErrorNotice error={error as Error}/>;}
  return <section aria-label="只读流程图" className="dag"><div className="dag-layers">
    {layers.map((layer,index)=><div className="dag-layer" key={index}><p>第 {index+1} 层</p>{layer.map(node=>{
      const state=nodes.find(item=>item.node_id===node.id);
      return <button key={node.id} className="dag-node" onClick={()=>document.getElementById(`node-${node.id}`)?.focus()}>
        <strong>{node.id}</strong><span>{node.executor}{node.read_only?' · 只读':''}</span><Status value={state?.status??'queued'}/>
        <small>依赖：{node.depends_on?.join('、')||'无'}</small></button>;
    })}</div>)}
  </div></section>;
}
