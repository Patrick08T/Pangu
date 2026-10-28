import {useRef,useState} from 'react';
import {JSONField,type Schema} from '../console/components';
import {NumberField,preciseJSON} from './number';
import {BriefInput} from './brief';

const briefNames:Record<string,string>={question:'研究问题',audience:'目标用户',market:'目标市场',time_scope:'时间范围',context:'现有资料',constraints:'约束条件',public_queries:'允许公开发送的搜索词',urls:'授权公开网址',documents:'授权项目相对文件',competitors:'竞品候选',dimensions:'比较维度'};
const textFields=['question','audience','market','time_scope','context','constraints'];
export const productBrief:Schema['Shape']={type:'object',required:['question'],properties:Object.fromEntries(Object.keys(briefNames).map(name=>[name,textFields.includes(name)?{type:'string'}:{type:'array',items:{type:'string'}}]))};
function empty(shape:Schema['Shape']):unknown {return shape.type==='object'?{}:shape.type==='array'?[]:shape.type==='boolean'?false:shape.type==='null'?null:shape.type==='number'||shape.type==='integer'?0:'';}

function ArrayField({shape,value,change,label,disabled}:{shape:Schema['Shape'];value:unknown;change:(value:unknown)=>void;label:string;disabled:boolean}) {
  const items=Array.isArray(value)?value:[],keys=useRef<string[]>([]);
  while(keys.current.length<items.length)keys.current.push(crypto.randomUUID());
  function remove(index:number){keys.current.splice(index,1);change(items.filter((_,i)=>i!==index));}
  return <fieldset disabled={disabled}><legend>{label}</legend>{items.map((item,index)=><div key={keys.current[index]}><InputField shape={shape} value={item} change={next=>change(items.map((old,i)=>i===index?next:old))} label={`${label} ${index+1}`} required disabled={disabled}/><button type="button" onClick={()=>remove(index)}>删除 {label} {index+1}</button></div>)}<button type="button" disabled={items.length>=100} onClick={()=>change([...items,empty(shape)])}>添加{label}</button></fieldset>;
}

export function InputField({shape,value,change,label,required=false,disabled=false}:{shape:Schema['Shape'];value:unknown;change:(value:unknown)=>void;label:string;required?:boolean;disabled?:boolean}) {
  if(shape.type==='object'){
    if(!required&&value===undefined)return <button type="button" disabled={disabled} onClick={()=>change({})}>填写{label}</button>;
    const fields=value&&typeof value==='object'&&!Array.isArray(value)?value as Record<string,unknown>:{};
    return <fieldset disabled={disabled}><legend>{label}</legend>{Object.entries(shape.properties??{}).map(([name,child])=><InputField key={name} shape={child} label={briefNames[name]??`${label}.${name}`} value={fields[name]} required={shape.required?.includes(name)} disabled={disabled} change={next=>{const updated={...fields};if(next===undefined)delete updated[name];else updated[name]=next;change(updated);}}/>)}{!required&&<button type="button" onClick={()=>change(undefined)}>省略{label}</button>}</fieldset>;
  }
  if(shape.type==='array'&&shape.items)return <ArrayField shape={shape.items} value={value} change={change} label={label} disabled={disabled}/>;
  if(shape.type==='boolean'||shape.type==='null')return <label className="field">{label}<select required={required} disabled={disabled} value={value===undefined?'':String(value)} onChange={e=>change(e.target.value===''?undefined:shape.type==='null'?null:e.target.value==='true')}><option value="">未填写</option>{shape.type==='null'?<option value="null">空值</option>:<><option value="true">是</option><option value="false">否</option></>}</select></label>;
  if(shape.type==='number'||shape.type==='integer')return <NumberField value={value} change={change} label={label} integer={shape.type==='integer'} required={required} disabled={disabled}/>;
  return <label className="field">{label}<textarea aria-required={required} disabled={disabled} value={typeof value==='string'?value:''} rows={3} onChange={e=>change(e.target.value)}/></label>;
}

function matches(shape:Schema['Shape'],value:unknown):boolean {
  if(value===undefined)return true;
  if(shape.type==='object')return !!value&&typeof value==='object'&&!Array.isArray(value)&&Object.entries(value).every(([name,item])=>!!shape.properties?.[name]&&matches(shape.properties[name],item));
  if(shape.type==='array')return Array.isArray(value)&&!!shape.items&&value.every(item=>matches(shape.items!,item));
  if(shape.type==='null')return value===null;
  return shape.type==='integer'?typeof value==='number'&&Number.isInteger(value):typeof value===shape.type;
}

export function RunInput({version,value,change,disabled=false}:{version:Pick<Schema['CatalogVersion'],'spec'>;value:string;change:(value:string)=>void;disabled?:boolean}) {
  const [advanced,setAdvanced]=useState(false);
  const shape=version.spec.flow?.parameters??(version.spec.agent?.role==='product_manager'?productBrief:undefined);
  if(!shape)return <label className="field task-goal">任务目标<textarea value={value} disabled={disabled} onChange={e=>change(e.target.value)} rows={5} placeholder="描述目标和希望得到的结果"/></label>;
  let parsed:unknown={},error='';
  try{parsed=value?JSON.parse(value):{};if(!matches(shape,parsed)||!preciseJSON(value))error='输入类型或数字精度不符合结构，请在高级输入中修正后切回表单。';}catch{error='输入不是有效 JSON，请在高级输入中修正后切回表单。';}
  return <>
    {advanced?<JSONField label="运行输入 JSON" value={value} onChange={change} disabled={disabled}/>:error?<p role="alert">{error}</p>:version.spec.agent?.role==='product_manager'?<BriefInput value={parsed as Record<string,unknown>} mode={version.spec.agent.mode??''} change={next=>change(JSON.stringify(next))} disabled={disabled}/>:<InputField label="运行参数" shape={shape} value={parsed} change={next=>change(JSON.stringify(next))} required disabled={disabled}/>}
    <details><summary>开发者输入</summary><label className="check"><input type="checkbox" checked={advanced} disabled={disabled} onChange={e=>setAdvanced(e.target.checked)}/>高级 JSON 输入</label></details>
  </>;
}
