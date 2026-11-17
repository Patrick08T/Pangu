import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,type Schema} from '../console/components';
import {receipt as receiptID} from '../runs/submission';
import {useDraftGuard} from './draft-guard';

export type DefaultEntryRunProps={project:string;csrf:string;available?:boolean;permitted?:boolean};
export function DefaultEntryRun(props:DefaultEntryRunProps){
  const [attempt,setAttempt]=useState(0),query=useQuery<Schema['ExperiencePolicy']>(props.available===true?`/projects/${encodeURIComponent(props.project)}/experience-policy`:null);
  if(props.available!==true)return <section><h2>工作台默认入口运行</h2><p className="notice">工作台默认入口运行尚未接通，不提供模拟提交；显式旧版本运行也不属于自动入口。</p></section>;
  return <section><h2>工作台默认入口运行</h2><p>使用当前项目已采用版本。角色/流程版本由服务在接收任务时选择并冻结；客户端不决定版本或修订，不替代原有业务审批。</p>
    <p className="notice">授权候选入口不代表当前已有绑定。提交时核验当前绑定；无绑定不可运行，也不会退回显式旧版本。</p><QueryState {...query} retry={query.reload}/>
    {!query.error&&!query.loading&&query.data&&<EntryForm key={`${props.project}:${attempt}`} {...props} entries={workbenchEntries(query.data.entrypoints)} revoked={query.data.revoked!==false} newRequest={()=>setAttempt(old=>old+1)}/>}
  </section>;
}

function workbenchEntries(values:unknown){
  if(!Array.isArray(values))return [];
  return [...new Set(values.filter((value):value is string=>typeof value==='string'&&value.startsWith('workbench:')).map(value=>value.slice('workbench:'.length)).filter(value=>value&&value.trim()===value&&!value.includes(':')))];
}

type FixedRequest={entry:string;input:string;key:string};
function EntryForm({entries,revoked,newRequest,...props}:DefaultEntryRunProps&{entries:string[];revoked:boolean;newRequest:()=>void}){
  const [entry,setEntry]=useState(''),[input,setInput]=useState(''),[confirmed,setConfirmed]=useState(false),[frozen,setFrozen]=useState(false),[receipt,setReceipt]=useState<Schema['Submission']>();
  const fixed=useRef<FixedRequest|null>(null),action=useAction(),allowed=props.permitted===true&&!!props.csrf&&!revoked&&entries.length>0;
  useDraftGuard(!receipt&&(input!==''||frozen));
  async function submit(signal:AbortSignal){
    if(!allowed||!confirmed||!entries.includes(entry)||!input.trim())throw new Error('请选择获准入口并填写输入、确认当前采用版本。');
    if(!fixed.current&&new TextEncoder().encode(JSON.stringify({input})).byteLength>256*1024)throw new Error('入口 JSON 请求体超过 256 KiB（UTF-8 字节），请缩短输入后重试；尚未发送请求。');
    fixed.current??={entry,input,key:crypto.randomUUID()};setFrozen(true);
    const request=fixed.current,value=await apiRequest<Schema['Submission']>(`/projects/${encodeURIComponent(props.project)}/entries/${encodeURIComponent(request.entry)}/runs`,{method:'POST',csrf:props.csrf,key:request.key,body:{input:request.input},signal});
    receiptID(value);
    if(value.project_id!==props.project||typeof value.task_id!=='string'||!value.task_id.trim())throw new Error('入口提交回执范围不符，结果未确认；保留原输入与请求键。');
    if(action.active())setReceipt(value);
  }
  if(receipt)return <div className="notice"><p>{receipt.status==='pending'?'请求已登记，尚未确认实际运行；请查看服务回执。':'入口请求已接受；不代表执行完成或审批通过。'}</p><a href={`#/requests/${encodeURIComponent(receipt.request_id)}`}>查看入口提交回执</a><button onClick={newRequest}>发起另一项入口任务</button></div>;
  return <form className="editor" onSubmit={event=>{event.preventDefault();void action.run(submit);}}>
    {props.permitted!==true&&<p className="notice">服务未授予当前用户项目运行权限，不能提交入口任务。</p>}{revoked&&<p className="notice">策略授权已撤销或无法确认，不能使用此自动入口。</p>}{entries.length===0&&<p className="notice">策略没有授权工作台入口；其他类型入口和显式版本不能替代。</p>}
    <label className="field">授权工作台入口<select value={entry} disabled={!allowed||action.busy||frozen} onChange={event=>setEntry(event.target.value)}><option value="">请选择入口</option>{entries.map(value=><option key={value} value={value}>{value}</option>)}</select></label>
    <label className="field">入口任务输入<textarea rows={6} value={input} disabled={!allowed||action.busy||frozen} onChange={event=>setInput(event.target.value)}/></label>
    <label className="check"><input type="checkbox" checked={confirmed} disabled={!allowed||action.busy||frozen} onChange={event=>setConfirmed(event.target.checked)}/>确认使用当前项目已采用版本，原有审批仍然有效</label>
    <button disabled={!allowed||!confirmed||!entry||!input.trim()||action.busy}>{frozen?'重试同一入口请求':'使用当前已采用版本运行'}</button>
    {frozen&&<p className="notice">输入已冻结；重试只使用同一入口、输入和请求键，不切换版本路径。请先核对本次结果，再发起另一项任务。</p>}<ActionNotice {...action}/>
  </form>;
}
