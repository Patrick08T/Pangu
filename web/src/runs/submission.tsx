import {useEffect,useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,JSONField,Page,QueryState,Status,canEdit,type Access,type Schema} from '../console/components';

export function receipt(value:Schema['Submission']) {
  if(!value||typeof value.request_id!=='string'||!(/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i).test(value.request_id)||!['pending','accepted'].includes(value.status))throw new APIError('提交回执不完整，操作结果尚未确认。请保留原输入重试。');
  return value.request_id;
}

export function RequestReceipt({id}:{id:string}) {
  const query=useQuery<Schema['Submission']>(`/requests/${id}`,5000);
  useEffect(()=>{if(query.data?.run_id)window.location.hash=`/runs/${query.data.run_id}`;},[query.data?.run_id]);
  return <div className="receipt"><h2>已保存请求</h2><p className="identifier">{id}</p><p>请求正在与调度服务对账，收到运行编号后自动进入详情。可保留此页面继续查询。</p>
    <QueryState {...query} retry={query.reload}/>{query.data&&<Status value={query.data.status}/>}</div>;
}

export function Start({access,version=''}:{access:Access;version?:string}) {
  const [mode,setMode]=useState(version?'version':'json'),[versionID,setVersion]=useState(version),[input,setInput]=useState('');
  const [definition,setDefinition]=useState(JSON.stringify({project_id:access.project,input:'',nodes:[]},null,2));
  const [request,setRequest]=useState<string>(),[check,setCheck]=useState<Schema['Preflight']>();
  const key=useRef(crypto.randomUUID()),action=useAction(),diagnose=useAction();
  const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500||action.error.status===409);
  const disabled=action.busy||diagnose.busy||uncertain||!canEdit(access);
  function rawDefinition(){const parsed=JSON.parse(definition) as Schema['Workflow'];if(parsed.project_id!==access.project)throw Error('定义中的 project_id 必须与当前项目一致。');return parsed;}
  async function submit(){
    const path=mode==='version'?`/versions/${encodeURIComponent(versionID)}/run`:'/runs';
    if(mode==='version'&&!versionID.trim())throw Error('请选择已发布的 Agent 或工作流版本。');
    if(mode==='version'){
      const fixed=await apiRequest<Schema['CatalogVersion']>(`/versions/${encodeURIComponent(versionID)}`);
      if(fixed.archived||!['agent','workflow'].includes(fixed.kind))throw Error('请选择可运行的发布版本。');
      if(!action.active())return;
    }
    const accepted=await apiRequest<Schema['Submission']>(path,{method:'POST',body:mode==='version'?{project_id:access.project,input}:rawDefinition(),csrf:access.session.csrf,key:key.current});
    setRequest(receipt(accepted));
  }
  async function validate(){
    setCheck(undefined);
    try{setCheck(await apiRequest<Schema['Preflight']>('/workflows/preflight',{method:'POST',body:rawDefinition(),csrf:access.session.csrf}));}
    catch(error){if(error instanceof APIError&&error.response&&typeof error.response==='object'&&'checks' in error.response)setCheck(error.response as Schema['Preflight']);throw error;}
  }
  return <Page title="发起运行" description="运行使用独立工作区，源项目文件保持原样。执行前由服务端检查权限、版本与 Worker。">
    {request?<RequestReceipt id={request}/>:<form className="editor" onSubmit={event=>{event.preventDefault();void action.run(submit,'请求已接受');}}>
      <label className="field">运行方式<select value={mode} disabled={disabled} onChange={e=>setMode(e.target.value)}><option value="json">JSON 编排</option><option value="version">已发布版本</option></select></label>
      {mode==='version'?<><label className="field">版本 ID<input value={versionID} disabled={disabled} onChange={e=>setVersion(e.target.value)} required/></label><label className="field">运行输入<textarea value={input} disabled={disabled} onChange={e=>setInput(e.target.value)} rows={10}/></label></>:<JSONField label="编排 JSON" value={definition} onChange={setDefinition} disabled={disabled} rows={18}/>}
      <div className="toolbar">{mode==='json'&&<button type="button" disabled={disabled} onClick={()=>void diagnose.run(validate,'执行前检查通过；配置就绪不代表模型调用已验证')}>执行前检查</button>}
        <button className="primary" disabled={action.busy||diagnose.busy||!canEdit(access)}>{uncertain?'重试提交':'提交运行'}</button><a href="#/runs">返回运行列表</a></div>
      {uncertain&&<p className="notice">输入已冻结，重试会使用同一个请求键。请先查清本次结果，再发起另一项工作。</p>}
      <ActionNotice {...action}/><ActionNotice {...diagnose}/>{check&&<pre>{JSON.stringify(check,null,2)}</pre>}
    </form>}
  </Page>;
}
