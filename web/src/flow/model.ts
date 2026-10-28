import type {Schema} from '../console/components';
import {dagLayers} from '../runs/workflow';

export type Flow=Schema['Flow'];
export type Step=Schema['FlowBinding'];
export type Versions=Record<string,Schema['CatalogVersion']>;
export type Issue={id?:string;message:string};
export const types=['string','number','integer','boolean','object','array','null'] as const;
export const typeNames:Record<string,string>={string:'文本',number:'数字',integer:'整数',boolean:'是 / 否',object:'对象',array:'列表',null:'空值'};
export function fieldName(pointer:string){return pointer?pointer.slice(1).split('/').map(token=>token.replaceAll('~1','/').replaceAll('~0','~')).join(' › '):'整个值';}
export const identifier=/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/;
export function label(flow:Flow,id:string){return flow.editor?.nodes?.[id]?.name||id;}
export function update(flow:Flow,id:string,patch:Partial<Step>):Flow{return {...flow,nodes:flow.nodes.map(node=>node.id===id?{...node,...patch}:node)};}
export function display(flow:Flow,id:string,patch:Partial<NonNullable<Schema['FlowEditor']['nodes']>[string]>):Flow {
  return {...flow,editor:{...flow.editor,nodes:{...flow.editor?.nodes,[id]:{x:0,y:0,...flow.editor?.nodes?.[id],...patch}}}};
}
export function connect(flow:Flow,from:string,to:string):Flow {
  if(from===to||!flow.nodes.some(n=>n.id===from)||!flow.nodes.some(n=>n.id===to))throw Error('不能连接自身或不存在的步骤');
  const target=flow.nodes.find(n=>n.id===to)!;
  if(target.depends_on?.includes(from))return flow;
  const next=update(flow,to,{depends_on:[...(target.depends_on??[]),from]});dagLayers(next.nodes);return next;
}
export function disconnect(flow:Flow,from:string,to:string){return update(flow,to,{depends_on:flow.nodes.find(n=>n.id===to)?.depends_on?.filter(id=>id!==from)});}
export function removeNodes(flow:Flow,ids:string[]):Flow {
  const nodes=flow.nodes.filter(n=>!ids.includes(n.id)).map(n=>({...n,depends_on:n.depends_on?.filter(id=>!ids.includes(id))}));
  const positions=Object.fromEntries(Object.entries(flow.editor?.nodes??{}).filter(([id])=>!ids.includes(id)));
  return {...flow,nodes,editor:{...flow.editor,nodes:positions}};
}
export function duplicate(flow:Flow,id:string,nextID:string):Flow {
  if(flow.nodes.length>=32)throw Error('最多添加 32 个步骤');
  const node=flow.nodes.find(n=>n.id===id);if(!node)return flow;
  const at=flow.editor?.nodes?.[id]??{x:0,y:0};
  return display({...flow,nodes:[...flow.nodes,structuredClone({...node,id:nextID})]},nextID,{name:`${label(flow,id)} 副本`,x:at.x+40,y:at.y+160});
}
export function arrange(flow:Flow):Flow {
  let layers:Step[][];try{layers=dagLayers(flow.nodes);}catch{layers=[flow.nodes];}
  const nodes={...flow.editor?.nodes};layers.forEach((layer,x)=>layer.forEach((node,y)=>{nodes[node.id]={...nodes[node.id],x:x*320+60,y:y*180+80};}));
  return {...flow,editor:{...flow.editor,nodes}};
}
export function issues(flow:Flow,versions?:Versions):Issue[] {
  const out:Issue[]=[];
  if(!flow.nodes.length)out.push({message:'添加至少一个助手步骤后再发布。'});
  try{dagLayers(flow.nodes);}catch(error){out.push({message:(error as Error).message});}
  for(const node of flow.nodes){
    const add=(message:string)=>out.push({id:node.id,message:`${label(flow,node.id)}：${message}`});
    if(!identifier.test(node.id))add('步骤标识无效');
    if(versions&&(!versions[node.agent_version]||versions[node.agent_version].archived))add('助手版本无法读取或已归档，请重新选择');
    for(const [name,binding] of Object.entries(node.bindings??{})){
      if(!identifier.test(name))add('映射参数名需以字母开头，只含字母、数字、下划线或短横线');
      if(binding.from!=='$input'&&!node.depends_on?.includes(binding.from))add(`参数 ${name} 的来源不是直接上游，请修复映射`);
      if(binding.from==='$input'&&!flow.parameters)add('引用流程输入前，请定义输入字段');
      const problem=bindingProblem(flow,node,binding,versions);if(problem)add(`参数 ${name}：${problem}`);
    }
    if(node.when){
      if(node.when.field.from!=='$input'&&!node.depends_on?.includes(node.when.field.from))add('执行条件引用的上游不存在');
      if(node.when.field.from==='$input'&&!flow.parameters)add('执行条件需要定义流程输入字段');
      const problem=bindingProblem(flow,node,node.when.field,versions);if(problem)add(`执行条件：${problem}`);
    }
    if(node.approval&&(node.approval.timeout_seconds<1||node.approval.timeout_seconds>3600))add('审批等待时间应为 1–3600 秒');
  }
  return out;
}

export function bindingProblem(flow:Flow,step:Step,binding:Schema['FieldBinding'],versions?:Versions){
  if(binding.pointer&&(!binding.pointer.startsWith('/')||/~(?![01])/u.test(binding.pointer)))return '字段路径格式无效';
  const parent=flow.nodes.find(n=>n.id===binding.from);
  let shape=binding.from==='$input'?flow.parameters:parent&&step.depends_on?.includes(parent.id)?(parent.output_schema??versions?.[parent.agent_version]?.spec.agent?.output_schema):undefined;
  if(!shape)return;
  for(const token of binding.pointer?binding.pointer.slice(1).split('/'):[]){
    const key=token.replaceAll('~1','/').replaceAll('~0','~');
    shape=shape.type==='object'?shape.properties?.[key]:shape.type==='array'&&/^(0|[1-9][0-9]*)$/.test(key)?shape.items:undefined;
    if(!shape)return '字段不存在，请重新选择';
  }
  if(shape.type!==binding.type&&!(shape.type==='integer'&&binding.type==='number'))return '引用字段的类型与声明不一致';
}

export function fields(shape:Schema['Shape']|undefined,pointer=''):{pointer:string;type:string}[] {
  if(!shape)return [];
  const result:{pointer:string;type:string}[]=[{pointer,type:shape.type}];
  for(const [name,child] of Object.entries(shape.properties??{}))result.push(...fields(child,`${pointer}/${name.replaceAll('~','~0').replaceAll('/','~1')}`));
  return result;
}
