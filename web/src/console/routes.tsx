import {Catalog} from '../catalog/list';
import {CatalogDetail} from '../catalog/version';
import Environment from '../environment/status';
import {Retention} from '../environment/retention';
import {Audit} from '../identity/audit';
import {Projects} from '../identity/project';
import {Tokens} from '../identity/token';
import {RunDetail} from '../runs/detail';
import {Runs} from '../runs/list';
import {RequestReceipt,Start} from '../runs/submission';
import {NewSchedule,ScheduleDetail,Schedules} from '../schedule/list';
import {isManager,type Access,type Schema} from './components';

const kinds=['template','agent','workflow','connection'];
export function Routes({route,access,projects,reload}:{route:string;access:Access;projects:Schema['Project'][];reload:()=>void}) {
  const [page,id,extra]=route.split('/').filter(Boolean);
  if(page==='environment')return <><Environment/>{access.session.admin&&<Retention access={access}/>}</>;
  if(page==='projects')return <Projects access={access} projects={projects} reload={reload}/>;
  if(page==='tokens')return <Tokens access={access}/>;
  if(!access.project)return <Projects access={access} projects={projects} reload={reload}/>;
  if(page==='audit')return isManager(access)?<Audit access={access}/>:<p>审计记录仅向管理者开放。</p>;
  if(page==='runs'&&id)return <RunDetail access={access} id={id}/>;
  if(page==='runs'||!page)return <Runs access={access}/>;
  if(page==='start')return <Start access={access} version={id}/>;
  if(page==='requests'&&id)return <RequestReceipt id={id}/>;
  if(page==='schedules')return <Schedules access={access}/>;
  if(page==='schedule'&&id==='new')return <NewSchedule access={access} version={extra}/>;
  if(page==='schedule'&&id)return <ScheduleDetail access={access} id={id}/>;
  if(page==='catalog'&&id)return <CatalogDetail access={access} id={id}/>;
  if(kinds.includes(page))return <Catalog access={access} kind={page}/>;
  return <p>页面不存在。<a href="#/runs">返回运行列表</a></p>;
}
