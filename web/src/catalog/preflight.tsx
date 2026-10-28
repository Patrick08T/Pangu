import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,type Access,type Schema} from '../console/components';

export function VersionPreflight({version,access}:{version:Schema['CatalogVersion'];access:Access}) {
  const action=useAction(),[input,setInput]=useState(''),[result,setResult]=useState<unknown>();
  async function check(){
    setResult(undefined);
    try{setResult(await apiRequest(`/versions/${version.id}/preflight`,{method:'POST',csrf:access.session.csrf,body:{input}}));}
    catch(error){if(error instanceof APIError&&error.response&&typeof error.response==='object'&&'checks' in error.response)setResult(error.response);throw error;}
  }
  return <section><h3>执行前诊断</h3><p className="muted">在指定 Worker 检查配置与环境，不发起收费模型请求。配置就绪不能证明凭据有效或模型服务可用。</p>
    {version.kind!=='connection'&&<label className="field">诊断用输入<textarea rows={5} disabled={action.busy} value={input} onChange={e=>setInput(e.target.value)}/></label>}
    <button disabled={action.busy} onClick={()=>void action.run(check,'环境诊断完成')}>检查 Worker 与凭据配置</button><ActionNotice {...action}/>{result!==undefined&&<pre>{JSON.stringify(result,null,2)}</pre>}
  </section>;
}
