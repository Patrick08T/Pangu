import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Page,QueryState,type Access,type Schema} from '../console/components';
import {PolicyEditor} from './policy';
import {Entries} from './entries';
import {EvolutionHistory} from './evolution';
import {CapabilityNotice} from './status';
import {RunApprovalGenerations} from './run';
import {RunLineage} from './lineage';
import {RunRebuilds} from './rebuild';
import {DefaultEntryRun} from './entry';
import './style.css';

type Capabilities=Schema['ExperienceCapabilities'];
const capabilitiesPath=(project:string)=>`/projects/${encodeURIComponent(project)}/experience-capabilities`;
export function ExperienceConsole({access}:{access:Access}){
  return <Page title="经验与进化" description="当前项目的来源授权、经验复用与受控晋级。默认关闭；所有操作都由服务重新核验权限、修订和证据。">
    {access.project?<ProjectConsole key={access.project} access={access}/>:<p className="notice">请先选择项目。</p>}
  </Page>;
}

function ProjectConsole({access}:{access:Access}){
  const query=useQuery<Capabilities>(capabilitiesPath(access.project)),[tab,setTab]=useState('entries'),[visited,setVisited]=useState(['entries']);
  const cap=!query.loading&&!query.error?query.data:undefined;
  return <div className="experience-console"><QueryState {...query} retry={query.reload}/>{cap&&<>
    <p>项目：{access.project}。未报告的权限或能力按不可用处理，状态不跨项目复用。</p>
    <nav className="toolbar" aria-label="经验管理分区">{[['entries','经验与来源'],['policy','策略与授权'],['evolution','实验与晋级'],['entry','默认入口运行']].map(([id,label])=><button key={id} aria-pressed={tab===id} onClick={()=>{setTab(id);setVisited(old=>old.includes(id)?old:[...old,id]);}}>{label}</button>)}</nav>
    <div hidden={tab!=='entries'}><Entries project={access.project} csrf={access.session.csrf??''} permissions={cap.permissions??{}}/></div>
    <div hidden={tab!=='policy'}>{visited.includes('policy')&&<ProjectPolicy access={access} cap={cap}/>}</div>
    <div hidden={tab!=='evolution'}>{visited.includes('evolution')&&(cap.permissions?.manage_policy===true?<EvolutionHistory project={access.project} csrf={access.session.csrf??''} rollback={cap.permissions?.rollback===true} available={cap.features?.evolution===true}/>:<p className="notice">服务未授予进化治理权限，不能查看本项目实验与晋级历史；不会发起历史查询。</p>)}</div>
    <div hidden={tab!=='entry'}>{visited.includes('entry')&&<DefaultEntryRun project={access.project} csrf={access.session.csrf??''} available={cap.features?.entry_submission===true} permitted={cap.permissions?.submit_entry===true}/>}</div>
  </>}</div>;
}

function ProjectPolicy({access,cap}:{access:Access;cap:Capabilities}){
  const query=useQuery<Schema['ExperiencePolicy']>(`/projects/${encodeURIComponent(access.project)}/experience-policy`);
  return <><QueryState {...query} retry={query.reload}/>{!query.error&&!query.loading&&query.data&&<PolicyEditor policy={query.data} project={access.project} csrf={access.session.csrf??''} permissions={cap.permissions??{}} features={cap.features??{}} supportedMetrics={cap.supported_metrics??[]}/>}
    <CapabilityNotice available={cap.features?.delivery_promotion===true} reason="经验投放晋级需要独立增益证据；未接通时不提供手动伪造验证回执或直接晋级操作。"/>
  </>;
}

export function ProjectExperienceLink(){
  return <section className="panel"><h2>项目经验与进化</h2><p>自动学习与运行读取分别授权。查看来源保留、验证状态、使用记录和发布观察；禁用不等于撤销授权。</p><a className="button" href="#/experience">管理项目经验与进化</a></section>;
}

export function RunExperience({project,run,csrf=''}:{project:string;run:string;csrf?:string}){
  const query=useQuery<Capabilities>(project?capabilitiesPath(project):null);
  return <section><h2>本次任务的经验依据</h2><QueryState {...query} retry={query.reload}/>{!query.error&&!query.loading&&query.data&&<>
    <p>运行：{run}</p>{query.data.features?.run_lineage===true?<RunLineage key={`lineage:${project}:${run}`} project={project} run={run}/>:<p className="notice">运行级影响链接口尚未接通，暂不能在此核对水位、匹配依据与实际注入哈希。不可将没有展示误认为没有使用经验。</p>}
    <p>经验详情中的使用摘要仅记录曾经使用，不证明当前依赖仍有效。重新审批只能由服务在旧执行停止后创建新代际，不提供模拟操作。</p><a href="#/experience">查看本项目经验与使用记录</a><RunApprovalGenerations key={`approvals:${project}:${run}`} project={project} run={run}/>
    {query.data.features?.approval_rebuild===true?<RunRebuilds key={`rebuild:${project}:${run}`} project={project} run={run} csrf={csrf}/>:<p>后继重建能力尚未确认；不提供重新审批操作。</p>}
  </>}</section>;
}
