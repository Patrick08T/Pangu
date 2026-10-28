export function toGraph(flow){
  return {
    nodes:flow.nodes.map(node=>({id:node.id,type:'assistant',meta:{position:{x:flow.editor?.nodes?.[node.id]?.x??0,y:flow.editor?.nodes?.[node.id]?.y??0}},data:{title:flow.editor?.nodes?.[node.id]?.name||node.id}})),
    edges:flow.nodes.flatMap(node=>(node.depends_on??[]).map(id=>({sourceNodeID:id,targetNodeID:node.id}))),
  };
}
export function fromGraph(base,graph){
  const ids=new Set(graph.nodes.map(node=>node.id));
  if(ids.size!==graph.nodes.length||graph.nodes.some(node=>!base.nodes.some(original=>original.id===node.id)))throw Error('节点标识无效');
  if(graph.edges.some(edge=>!ids.has(edge.sourceNodeID)||!ids.has(edge.targetNodeID)))throw Error('连线引用无效');
  const nodes=base.nodes.filter(node=>ids.has(node.id)).map(node=>{
    const parents=graph.edges.filter(edge=>edge.targetNodeID===node.id).map(edge=>edge.sourceNodeID);
    return parents.length||node.depends_on?{...node,depends_on:parents}:node;
  });
  const positions=Object.fromEntries(graph.nodes.map(node=>[node.id,{...base.editor?.nodes?.[node.id],...node.meta.position}]));
  return {...base,nodes,editor:{...base.editor,nodes:positions}};
}
