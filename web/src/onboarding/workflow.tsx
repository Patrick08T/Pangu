import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,Page,QueryState,canEdit,type Access,type Schema} from '../console/components';
import {RequestReceipt,receipt} from '../runs/submission';
import {AssistantPicker,AssistantScope,PreflightStatus} from './catalog';
import {RunInput} from './input';

export function Onboarding({access,versionID=''}:{access:Access;versionID?:string}) {
  const [selected,setSelected]=useState<Schema['CatalogVersion']>(),[input,setInput]=useState('');
  const [diagnostic,setDiagnostic]=useState<Schema['Preflight']>(),[request,setRequest]=useState(''),[stage,setStage]=useState('');
  const fixed=useQuery<Schema['CatalogVersion']>(versionID?`/versions/${encodeURIComponent(versionID)}`:null);
  const key=useRef(crypto.randomUUID()),submitted=useRef(false),action=useAction();
  const version=fixed.data??selected;
  const uncertain=submitted.current&&action.error instanceof APIError&&(action.error.status===0||action.error.status>=500||action.error.status===409);
  const disabled=action.busy||uncertain||!canEdit(access);
  const scoped=version&&!version.archived&&['agent','workflow'].includes(version.kind);
  function edit(next:string){setInput(next);setDiagnostic(undefined);key.current=crypto.randomUUID();}
  function select(next:Schema['CatalogVersion']|undefined){setSelected(next);edit('');}
  async function submit(signal:AbortSignal){
    if(!version||!scoped)throw Error('请选择已发布的角色或工作流。');
    if(!uncertain){
      submitted.current=false;setDiagnostic(undefined);setStage('正在检查任务与环境…');
      try {
        const result=await apiRequest<Schema['Preflight']>(`/versions/${version.id}/preflight`,{method:'POST',csrf:access.session.csrf,body:{project_id:access.project,input},signal});
        if(!action.active())return;setDiagnostic(result);
        if(!result.ready)throw Error('任务检查未通过，请按提示补充配置。');
      }catch(error){
        if(action.active()&&error instanceof APIError&&error.response&&typeof error.response==='object'&&'checks' in error.response){setDiagnostic(error.response as Schema['Preflight']);throw Error('请处理下方缺项后重新开始。');}
        throw error;
      }
    }
    if(!action.active())return;
    setStage('正在创建任务…');submitted.current=true;
    const accepted=await apiRequest<Schema['Submission']>(`/versions/${version.id}/run`,{method:'POST',csrf:access.session.csrf,key:key.current,body:{project_id:access.project,input},signal});
    if(action.active())setRequest(receipt(accepted));
  }
  return <Page title="新建任务" description="选择助手，描述你想完成的事。执行结果会保存在任务记录中。">
    {!access.project?<div className="empty"><h2>先选择一个项目</h2><p>项目决定本次任务使用的代码和执行环境。</p><a className="button primary" href="#/projects">登记或查看项目</a></div>:request?<RequestReceipt id={request}/>:<form className="editor task-composer" onSubmit={e=>{e.preventDefault();void action.run(submit);}}>
      {!canEdit(access)&&<p className="notice">你可以查看助手；开始任务需要项目操作者权限，请联系项目管理者。</p>}
      {versionID?<QueryState {...fixed} retry={fixed.reload}/>:<AssistantPicker value={selected?.id??''} change={select} disabled={disabled}/>}
      {version&&!scoped&&<p role="alert">此版本已归档或不可运行，请重新选择助手。</p>}
      {version&&scoped&&<><AssistantScope version={version}/><RunInput key={version.id} version={version} value={input} change={edit} disabled={disabled}/>
        <div className="task-submit"><button className="primary" disabled={action.busy||!canEdit(access)}>{action.busy?stage:uncertain?'重试同一任务':'开始执行'}</button><small>开始前自动检查；通过后会调用所选助手。</small></div>
        <details><summary>查看实际任务输入</summary><pre>{input||'（空输入）'}</pre></details></>}
      {uncertain&&<p className="notice">提交结果尚未确认。输入已保留并锁定，重试会查询或继续同一任务，不重复创建。</p>}
      <ActionNotice {...action}/>{diagnostic&&<PreflightStatus result={diagnostic}/>}
    </form>}
  </Page>;
}