import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';

export function CodexModelPicker({value,change,disabled,editable}:{value:string;change:(model:string)=>void;disabled:boolean;editable:boolean}){
  const query=useQuery<Schema['ModelChoicePage']>(editable?'/models/codex':null);
  const items=query.data?.items??[];
  return <><label className="field">Codex 模型<select value={value} disabled={disabled||!editable||query.loading} onChange={e=>change(e.target.value)}>
    <option value="">Worker 默认模型</option>
    {value&&!items.some(item=>item.id===value)&&<option value={value}>当前模型：{value}</option>}
    {items.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}
  </select></label>{editable&&<><QueryState {...query} retry={query.reload}/><button type="button" disabled={disabled||query.loading} onClick={query.reload}>刷新模型列表</button><p className="muted">候选来自 Worker 当前 Codex 程序的内置模型目录。</p></>}</>;
}
