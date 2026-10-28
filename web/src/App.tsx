import {useEffect,useState} from 'react';
import {useAction,useQuery} from './api/hooks';
import {apiRequest,APIError} from './api/request';
import {ActionNotice,ErrorNotice,QueryState,type Access,type Schema,type Session} from './console/components';
import {Routes} from './console/routes';

function Login({error,retry}:{error?:Error;retry:()=>void}) {
  const unavailable=!!error&&(!(error instanceof APIError)||error.status!==401);
  return <main className="login"><a className="brand" href="/"><span>P</span>PANGU</a><p className="eyebrow">AGENT 工厂</p><h1>让任务有序执行</h1><p className="intro">通过对话创建助手与工作流，并追踪每一次执行。</p>
    {unavailable?<ErrorNotice error={error} retry={retry}/>:<><p>使用已登记的飞书身份进入工作空间。</p><a className="button primary" href="/api/auth/login">使用飞书登录</a></>}
    <p className="muted">首次使用需要维护者配置飞书应用，并由管理员登记项目成员。</p></main>;
}

export default function App() {
  const query=useQuery<Session>('/auth/me'),[expired,setExpired]=useState(false);
  useEffect(()=>{const expire=()=>setExpired(true);window.addEventListener('pangu:unauthenticated',expire);return()=>window.removeEventListener('pangu:unauthenticated',expire);},[]);
  if(expired)return <Login retry={()=>{setExpired(false);query.reload();}}/>;
  if(query.loading)return <main className="login"><p role="status">正在确认登录状态…</p></main>;
  if(!query.data)return <Login error={query.error} retry={query.reload}/>;
  return <Workspace session={query.data} logout={()=>setExpired(true)}/>;
}

function Workspace({session,logout}:{session:Session;logout:()=>void}) {
  const projects=useQuery<Schema['Project'][]>('/projects',30000),memberships=useQuery<Schema['Membership'][]>('/auth/memberships',30000);
  const [selected,setSelected]=useState(''),[route,setRoute]=useState(window.location.hash.slice(1)||'/factory'),action=useAction();
  useEffect(()=>{const change=()=>{if(leaveAllowed())setRoute(window.location.hash.slice(1)||'/factory');else window.history.replaceState(null,'',`#${route}`);};window.addEventListener('hashchange',change);return()=>window.removeEventListener('hashchange',change);},[route]);
  const project=projects.data?.some(item=>item.id===selected)?selected:projects.data?.[0]?.id??'';
  const access:Access={session,project,role:session.admin?'manager':memberships.data?.find(item=>item.project_id===project)?.role??'',catalogManager:session.admin||memberships.data?.some(item=>item.role==='manager')};
  const nav=[['factory','工厂助手'],['runs','任务记录'],['agent','Agent 角色'],['workflow','工作流'],['schedules','自动化']];
  async function signOut(){if(!leaveAllowed())return;await apiRequest('/auth/logout',{method:'POST',csrf:session.csrf});logout();}
  return <div className="workspace"><aside><a className="brand" href="#/factory"><span>P</span>PANGU</a>
    <label className="field project-select">当前项目<select aria-label="当前项目" value={project} onChange={e=>{if(leaveAllowed())setSelected(e.target.value);}}>{!projects.data?.length&&<option value="">尚无项目</option>}{projects.data?.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
    <nav aria-label="主导航">{nav.map(([path,label])=><a key={path} href={`#/${path}`} className={route.split('/')[1]===path?'nav-current':''} aria-current={route.split('/')[1]===path?'page':undefined}>{label}</a>)}</nav>
    <a className={`settings-link ${route==='/settings'?'nav-current':''}`} href="#/settings">设置</a>
    <div className="sidebar-note"><strong>{session.name}</strong><p>{session.admin?'工作空间管理员':access.role==='manager'?'项目管理者':access.role==='operator'?'项目操作者':'项目查看者'}</p><button disabled={action.busy} onClick={()=>void action.run(signOut)}>退出登录</button><ActionNotice {...action}/></div>
    </aside><main><a className="skip-nav" href="#content" onClick={e=>{e.preventDefault();document.getElementById('content')?.focus();}}>跳转到内容</a><QueryState loading={projects.loading||memberships.loading} error={projects.error??memberships.error} retry={()=>{projects.reload();memberships.reload();}}/>
      {projects.data&&memberships.data&&<div id="content" tabIndex={-1} key={`${project}:${route}`}><Routes route={route} access={access} projects={projects.data} reload={()=>{projects.reload();memberships.reload();}}/></div>}
    </main></div>;
}

function leaveAllowed(){return window.dispatchEvent(new Event('pangu:before-navigate',{cancelable:true}));}
