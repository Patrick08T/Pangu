import {JSONField,type Schema} from '../console/components';
import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState} from '../console/components';

export function ConnectionPicker({value,change,disabled=false}:{value:string;change:(value:string,model:string)=>void;disabled?:boolean}) {
  const [offset,setOffset]=useState(0);
  const query=useQuery<Schema['ConnectionChoicePage']>(`/connections/choices?${new URLSearchParams({offset:String(offset)})}`);
  function page(next:number){setOffset(next);}
  return <fieldset disabled={disabled}><legend>模型连接</legend><QueryState {...query} retry={query.reload}/>
    <label className="field">共享连接版本<select required value={value} disabled={disabled||query.loading||!!query.error} onChange={e=>change(e.target.value,query.data?.items.find(item=>item.id===e.target.value)?.model??'')}><option value="">选择连接及固定版本</option>{value&&!query.data?.items.some(item=>item.id===value)&&<option value={value}>当前固定版本（不在本页） · {value}</option>}{query.data?.items.map(item=><option key={item.id} value={item.id}>{item.name} · v{item.number} · {item.model||'Codex 默认模型'}</option>)}</select></label>
    {(offset>0||query.data?.next_offset!==undefined)&&<div className="toolbar"><button type="button" disabled={!offset} onClick={()=>page(0)}>连接首页</button><button type="button" disabled={query.data?.next_offset===undefined} onClick={()=>page(query.data!.next_offset!)}>更多连接</button></div>}
  </fieldset>;
}

export function ConnectionFields({value,change,disabled=false}:{value:string;change:(value:string)=>void;disabled?:boolean}) {
  const [advanced,setAdvanced]=useState(false);
  let spec:Schema['CatalogSpec']={};let invalid=false;
  try{
    const parsed:unknown=JSON.parse(value);
    if(!parsed||typeof parsed!=='object'||Array.isArray(parsed))throw Error('对象');
    spec=parsed as Schema['CatalogSpec'];
    const connection=spec.connection;
    if(connection!==undefined&&(!connection||typeof connection!=='object'||Array.isArray(connection)||['provider','base_url','model','secret_ref'].some(key=>connection[key as 'model']!==undefined&&typeof connection[key as 'model']!=='string')))throw Error('连接字段');
  }catch{invalid=true;}
  const codex=spec.connection?.provider==='codex';
  function update(key:'base_url'|'model'|'secret_ref',next:string){change(JSON.stringify({...spec,connection:{...spec.connection,[key]:next}},null,2));}
  function provider(next:string){change(JSON.stringify({...spec,connection:next==='codex'?{provider:'codex',model:''}:{provider:'openai',base_url:'',model:'',secret_ref:''}},null,2));}
  return <><label><input type="checkbox" checked={advanced} disabled={disabled} onChange={e=>setAdvanced(e.target.checked)}/>高级连接 JSON</label>
    {advanced?<JSONField label="配置 JSON" value={value} onChange={change} disabled={disabled}/>:invalid?<p role="alert">配置必须是 JSON 对象，连接字段必须为字符串；请在高级编辑中修正。</p>:<>
      <label className="field">连接方式<select disabled={disabled} value={spec.connection?.provider??'openai'} onChange={e=>provider(e.target.value)}><option value="openai">模型 API</option><option value="codex">Worker 的 Codex 账号</option></select></label>
      <label className="field">模型名称<input required={!codex} maxLength={128} disabled={disabled} value={spec.connection?.model??''} onChange={e=>update('model',e.target.value)}/></label>
      {codex?<p className="muted">共用 Worker 已登录的专用 Codex 账号；模型留空使用 Codex 默认模型。工具由 Eino 执行，每次模型回复计入 Codex 会话预算，费用显示未知。</p>:<>
      <label className="field">模型服务地址<input type="url" required disabled={disabled} value={spec.connection?.base_url??''} onChange={e=>update('base_url',e.target.value)}/></label>
      <label className="field">服务端凭据引用<input required pattern="PANGU_SECRET_[A-Z][A-Z0-9_]{0,63}" disabled={disabled} value={spec.connection?.secret_ref??''} onChange={e=>update('secret_ref',e.target.value)}/></label>
      <p className="muted">填写维护者提供的 PANGU_SECRET_ 引用名，勿粘贴密钥。</p>
      <p>可在高级 JSON 中登记 price 单价；未登记时费用显示未知。单价以每百万 token 的微货币单位表示，1 元或 1 美元 = 1000000 微单位。</p>
      </>}
    </>}
  </>;
}
