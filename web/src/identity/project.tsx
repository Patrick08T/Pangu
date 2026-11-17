import {lazy,Suspense,useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,isManager,type Access,type Schema} from '../console/components';
import {Members} from './member';
const ProjectExperienceLink=lazy(()=>import('../experience/console').then(module=>({default:module.ProjectExperienceLink})));

export function Projects({access,projects,reload}:{access:Access;projects:Schema['Project'][];reload:()=>void}) {
  const project=projects.find(item=>item.id===access.project);
  return <Page title="项目与成员" description="项目绑定本地目录和指定 Worker。Git 在独立副本执行；Perforce 直接在绑定的 P4 工作区执行。">
    {project&&<section className="panel"><h2>{project.name}</h2><dl><dt>项目 ID</dt><dd className="identifier">{project.id}</dd><dt>来源类型</dt><dd>{project.vcs==='perforce'?'Perforce':'Git'}</dd>{project.branch&&<><dt>本地分支</dt><dd>{project.branch}</dd></>}<dt>Worker</dt><dd>{project.worker_id}</dd>{project.source&&<><dt>登记来源</dt><dd>{project.source}</dd></>}{project.perforce&&<><dt>Perforce 服务器</dt><dd>{project.perforce.port}</dd><dt>Client 与范围</dt><dd>{project.perforce.client} / {project.perforce.scope}</dd></>}</dl></section>}
    {project?.vcs==='perforce'&&access.session.admin&&<PerforceBinding key={project.id} project={project} access={access} reload={reload}/>}
    {project?.verification?.length?<section><h2>验证命令</h2><pre>{JSON.stringify(project.verification,null,2)}</pre><p>每个代码节点成功后执行，真实退出证据保存在节点尝试中。</p></section>:project&&<p>未登记自动验证命令。代码产物仍需人工审查。</p>}
    {project&&<Suspense fallback={<p role="status">正在打开经验入口…</p>}><ProjectExperienceLink/></Suspense>}
    {isManager(access)&&access.project&&<Members key={access.project} access={access}/>}
    {access.session.admin&&<RegisterProject access={access} reload={reload}/>}
    {!project&&!access.session.admin&&<p>尚未获得项目权限，请联系项目管理者登记飞书身份。</p>}
  </Page>;
}

function RegisterProject({access,reload}:{access:Access;reload:()=>void}) {
  const [value,setValue]=useState({name:'',source:'',branch:'main',worker_id:''}),[vcs,setVCS]=useState<'git'|'perforce'>('git'),[perforce,setPerforce]=useState<Schema['PerforceConfig']>({port:'',user:'',client:'',scope:''}),[verification,setVerification]=useState('[]'),action=useAction();
  const labels={name:'项目名称',worker_id:'Worker ID',source:vcs==='git'?'Worker 上的 Git 项目绝对路径':'本地 P4 工作区根目录',...(vcs==='git'?{branch:'本地分支（不自动拉取）'}:{})};
  function body():Schema['ProjectRegistration']{const args:unknown=JSON.parse(verification);if(!Array.isArray(args)||args.some(item=>typeof item!=='string'))throw new Error('验证命令必须是字符串数组');return {name:value.name,worker_id:value.worker_id,vcs,source:value.source,verification:args,...(vcs==='git'?{branch:value.branch}:{perforce})};}
  async function register(){await apiRequest('/projects',{method:'POST',csrf:access.session.csrf,body:body()});if(action.active())reload();}
  async function check(){const result=await apiRequest<Schema['Preflight']>('/projects/check',{method:'POST',csrf:access.session.csrf,body:body()});if(!result.ready)throw new Error('Perforce 连接检查未通过');}
  return <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(register,'项目已登记');}}><fieldset disabled={action.busy}><legend>登记项目</legend>
    <label className="field">来源类型<select value={vcs} onChange={e=>setVCS(e.target.value as typeof vcs)}><option value="git">Git</option><option value="perforce">Perforce</option></select></label>
    {(Object.keys(labels) as (keyof typeof labels)[]).map(key=><label className="field" key={key}>{labels[key]}<input required value={value[key]} onChange={e=>setValue({...value,[key]:e.target.value})}/></label>)}
    {vcs==='perforce'&&<><PerforceFields value={perforce} change={setPerforce}/><p>直接在绑定目录的项目子目录中执行。保留已有修改和 changelist 归属，执行前记录快照；新打开文件进入本轮 changelist，不自动提交、同步或回退文件。</p><button type="button" onClick={()=>void action.run(check,'本地目录、Client 映射和连接检查通过')}>检查 Perforce 连接</button></>}
    <label className="field">验证命令 JSON 参数数组<textarea value={verification} onChange={e=>setVerification(e.target.value)} spellCheck={false}/></label><p>可选，空数组表示不运行。第一项为 Worker 上已安装程序的绝对路径，后续项为参数；每次最多 30 秒，不填写密钥。程序在节点的执行目录内启动。</p><button className="primary">登记项目</button>
  </fieldset><ActionNotice {...action}/></form>;
}

function PerforceFields({value,change}:{value:Schema['PerforceConfig'];change:(value:Schema['PerforceConfig'])=>void}){
  const fields={port:'Perforce 服务器',user:'Perforce 用户',client:'P4 Client',scope:'项目子目录'};
  return <>{(Object.keys(fields) as (keyof typeof fields)[]).map(key=><label className="field" key={key}>{fields[key]}<input required value={value[key]} onChange={e=>change({...value,[key]:e.target.value})}/></label>)}<p>服务器格式为 host:port 或 ssl:host:port。项目子目录相对于本地工作区根目录；填写 . 选择全部映射，建议选择代码子目录以控制快照与修改范围。</p></>;
}

function PerforceBinding({project,access,reload}:{project:Schema['Project'];access:Access;reload:()=>void}){
  const [source,setSource]=useState(project.source??''),action=useAction();
  async function save(){await apiRequest(`/projects/${project.id}/workspace`,{method:'PATCH',csrf:access.session.csrf,body:{source}});if(action.active())reload();}
  return <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(save,'本地 P4 工作区已绑定');}}><fieldset disabled={action.busy}><legend>Perforce 本地工作区</legend><label className="field">绑定的本地 P4 工作区根目录<input required value={source} onChange={e=>setSource(e.target.value)}/></label><p>填写 Worker 上 Client 的 Root 或 AltRoot。任务直接在该目录的项目子目录中读写，保留现有未提交修改；失败不自动回退。绑定前会核对本地目录与 Client 映射。</p><button disabled={source===project.source}>检查并保存目录绑定</button></fieldset><ActionNotice {...action}/></form>;
}
