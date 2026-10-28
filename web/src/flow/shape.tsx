import type {Schema} from '../console/components';
import {types} from './model';

const typeNames={string:'文本',number:'数值',integer:'整数',boolean:'是 / 否',object:'对象',array:'列表',null:'空值'};
export function ShapeFields({value,change,disabled,depth=0,title='流程输入字段'}:{value:Schema['Shape'];change:(value:Schema['Shape'])=>void;disabled:boolean;depth?:number;title?:string}) {
  function property(name:string,next:Schema['Shape']){change({...value,properties:{...value.properties,[name]:next}});}
  function rename(name:string,next:string){if(next!==name&&Object.hasOwn(value.properties??{},next))return;change({...value,properties:Object.fromEntries(Object.entries(value.properties??{}).map(([key,item])=>[key===name?next:key,item])),required:value.required?.map(key=>key===name?next:key)});}
  function add(){let i=1;while(value.properties?.[`field${i}`])i++;property(`field${i}`,{type:'string'});}
  return <div className="flow-shape"><fieldset disabled={disabled}><legend>{depth?'字段结构':title}</legend>
    {Object.entries(value.properties??{}).map(([name,item],index)=><div className="flow-property" key={index}>
      <div className="flow-property-row"><label>字段名<input aria-label={`字段名 ${depth}-${index+1}`} value={name} maxLength={64} onChange={e=>rename(name,e.target.value)}/></label>
        <label>类型<select aria-label={`字段类型 ${name}`} value={item.type} onChange={e=>property(name,{type:e.target.value as Schema['Shape']['type'],...(e.target.value==='array'?{items:{type:'string' as const}}:{})})}>{types.map(type=><option key={type} value={type}>{typeNames[type]}</option>)}</select></label>
        <label className="check"><input type="checkbox" checked={value.required?.includes(name)??false} onChange={e=>change({...value,required:e.target.checked?[...(value.required??[]),name]:(value.required??[]).filter(key=>key!==name)})}/>必填</label>
        <button type="button" aria-label={`删除字段 ${name}`} onClick={()=>change({...value,properties:Object.fromEntries(Object.entries(value.properties??{}).filter(([key])=>key!==name)),required:value.required?.filter(key=>key!==name)})}>删除</button></div>
      {item.type==='object'&&depth<6&&<ShapeFields value={item} change={next=>property(name,next)} disabled={disabled} depth={depth+1}/>}
      {item.type==='array'&&<ArrayFields value={item} change={next=>property(name,next)} disabled={disabled} depth={depth+1}/>}
    </div>)}
    <button type="button" disabled={Object.keys(value.properties??{}).length>=64||depth>=6} onClick={add}>添加字段</button>
  </fieldset></div>;
}
function ArrayFields({value,change,disabled,depth}:{value:Schema['Shape'];change:(value:Schema['Shape'])=>void;disabled:boolean;depth:number}) {
  const item=value.items??{type:'string'};
  return <div className="flow-array"><label className="field">列表项类型<select value={item.type} disabled={disabled||depth>=6} onChange={e=>change({...value,items:{type:e.target.value as Schema['Shape']['type'],...(e.target.value==='array'?{items:{type:'string' as const}}:{})}})}>{types.map(type=><option key={type} value={type}>{typeNames[type]}</option>)}</select></label>
    {item.type==='object'&&depth<6&&<ShapeFields value={item} change={next=>change({...value,items:next})} disabled={disabled} depth={depth}/>}
    {item.type==='array'&&depth<6&&<ArrayFields value={item} change={next=>change({...value,items:next})} disabled={disabled} depth={depth+1}/>}
  </div>;
}
