import type {Schema} from '../console/components';
import {handoffProblems} from './handoff';
export type Agent=Schema['Node'];
export type Draft=Pick<Schema['CatalogObject'],'name'|'spec'>;
export const modeNames:Record<string,string>={research:'资料调研',competition:'竞品分析',planning:'产品规划',design:'方案设计'};
export const engine=(node:Agent)=>node.executor==='codex'?'codex':node.role==='product_manager'?'product_manager':'eino';
export const engineName=(node:Agent)=>node.executor==='codex'?'原生 Codex':node.role==='product_manager'?`Eino 产品经理 · ${modeNames[node.mode??'']??'未选择能力'}`:'Eino 通用助手';
export const policy=(node:Agent)=>node.policy??{retries:2};
export function changeEngine(node:Agent,next:string):Agent{
  const {role:_,mode:__,connection_version:___,model:____,read_only:_____,...common}=node;
  return {...common,executor:next==='codex'?'codex':'eino',...(next==='codex'?{read_only:true}:node.executor==='eino'?{connection_version:node.connection_version,model:node.model}:{}),...(next==='product_manager'?{role:'product_manager',mode:'planning'}:{})};
}
export function problems(draft:Draft){
  const node=draft.spec.agent,issues:string[]=[],bytes=(value:string)=>new TextEncoder().encode(value).length;
  if(!draft.name.trim()||bytes(draft.name)>128)issues.push('助手名称必填，最多 128 字节。');
  if(!node)return [...issues,'缺少助手配置。'];
  issues.push(...handoffProblems(node.handoff,false));
  if(!node.instruction.trim()||bytes(node.instruction)>16384)issues.push('助手职责必填，最多 16384 字节。');
  if(bytes(node.description??'')>1024)issues.push('用途说明最多 1024 字节。');
  const p=policy(node);
  if(!Number.isInteger(p.retries)||p.retries<0||p.retries>2)issues.push("重试次数必须为 0–2。");
  if(node.output_schema&&invalidShape(node.output_schema))issues.push('输出结构包含无效字段名或不完整的字段类型。');
  return issues;
}
function invalidShape(shape:Schema['Shape'],depth=0):boolean{
  if(depth>6)return true;
  if(shape.type==='array')return !shape.items||invalidShape(shape.items,depth+1);
  if(shape.type==='object')return Object.entries(shape.properties??{}).some(([key,value])=>!key.trim()||new TextEncoder().encode(key).length>64||invalidShape(value,depth+1));
  return !['string','number','integer','boolean','null'].includes(shape.type);
}
const labels:Record<string,string>={handoff:'交接契约',instruction:'助手职责',description:'用途说明',executor:'执行器',role:'角色',mode:'产品能力',connection_version:'模型连接版本',model:'模型',policy:'重试策略',output_schema:'输出字段',read_only:'只读模式',approval:'审批要求',id:'节点标识'};
export function differences(before:Schema['CatalogSpec'],after:Schema['CatalogSpec']){
  const a=before.agent??{},b=after.agent??{};
  return [...new Set([...Object.keys(a),...Object.keys(b)])].filter(key=>JSON.stringify(a[key as keyof typeof a])!==JSON.stringify(b[key as keyof typeof b])).map(key=>({name:labels[key]??key,before:format(a[key as keyof typeof a]),after:format(b[key as keyof typeof b])}));
}
function format(value:unknown){return value===undefined?'未设置':typeof value==='string'?value:typeof value==='boolean'?(value?'是':'否'):JSON.stringify(value,null,2);}
