import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,isManager,type Access,type Schema} from '../console/components';
import {Members} from './member';

export function Projects({access,projects,reload}:{access:Access;projects:Schema['Project'][];reload:()=>void}) {
  const project=projects.find(item=>item.id===access.project);
  return <Page title="项目与成员" description="项目绑定本地 Git 来源和指定 Worker；运行只修改各自独立工作区。">
    {project&&<section className="panel"><h2>{project.name}</h2><dl><dt>项目 ID</dt><dd className="identifier">{project.id}</dd><dt>本地分支</dt><dd>{project.branch}</dd><dt>Worker</dt><dd>{project.worker_id}</dd>{project.source&&<><dt>登记来源</dt><dd>{project.source}</dd></>}</dl></section>}
    {project?.verification?.length?<section><h2>验证命令</h2><pre>{JSON.stringify(project.verification,null,2)}</pre><p>每个代码节点成功后执行，真实退出证据保存在节点尝试中。</p></section>:project&&<p>未登记自动验证命令。代码产物仍需人工审查。</p>}
    {isManager(access)&&access.project&&<Members key={access.project} access={access}/>}
    {access.session.admin&&<RegisterProject access={access} reload={reload}/>}
    {!project&&!access.session.admin&&<p>尚未获得项目权限，请联系项目管理者登记飞书身份。</p>}
  </Page>;
}

function RegisterProject({access,reload}:{access:Access;reload:()=>void}) {
  const [value,setValue]=useState({name:'',source:'',branch:'main',worker_id:''}),[verification,setVerification]=useState('[]'),action=useAction();
  const labels={name:'项目名称',source:'Worker 上的 Git 项目绝对路径',branch:'本地分支（不自动拉取）',worker_id:'Worker ID'};
  async function register(){const args:unknown=JSON.parse(verification);if(!Array.isArray(args)||args.some(item=>typeof item!=='string'))throw new Error('验证命令必须是字符串数组');await apiRequest('/projects',{method:'POST',csrf:access.session.csrf,body:{...value,verification:args}});if(action.active())reload();}
  return <form className="editor" onSubmit={e=>{e.preventDefault();void action.run(register,'项目已登记');}}><fieldset disabled={action.busy}><legend>登记项目</legend>
    {(Object.keys(labels) as (keyof typeof labels)[]).map(key=><label className="field" key={key}>{labels[key]}<input required value={value[key]} onChange={e=>setValue({...value,[key]:e.target.value})}/></label>)}
    <label className="field">验证命令 JSON 参数数组<textarea value={verification} onChange={e=>setVerification(e.target.value)} spellCheck={false}/></label><p>可选，空数组表示不运行。第一项为 Worker 上已安装程序的绝对路径，后续项为参数；每次最多 30 秒，不填写密钥。程序在私有运行副本内启动。</p><button className="primary">登记项目</button>
  </fieldset><ActionNotice {...action}/></form>;
}
