import type {Schema} from '../console/components';

const labels={not_assessed:'尚无业务验收结论',pending:'业务验收待完成',accepted:'业务验收通过',needs_attention:'业务验收未通过',continued:'已转入新版本续跑'};

export function OutcomeLabel({outcome}:{outcome?:Schema['BusinessOutcome']}){
  if(!outcome)return null;
  return <span className={`status ${outcome.state==='accepted'?'status-succeeded':'status-unknown'}`}>{labels[outcome.state]}</span>;
}

export function OutcomeDetails({outcome}:{outcome?:Schema['BusinessOutcome']}){
  if(!outcome)return null;
  return <section className="notice" aria-label="业务验收结果"><OutcomeLabel outcome={outcome}/>
    {outcome.state==='needs_attention'&&<p>执行步骤已返回结果，验收条件仍未全部满足。请根据以下结论补证或修订，再定向复验。</p>}
    {outcome.state==='not_assessed'&&<p>尚未取得明确的终端业务验收结论，执行完成不代表目标已经达成。</p>}
    {outcome.nodes.map(n=>n.summary&&<p key={n.node_id}><OutcomeSummary text={n.summary}/></p>)}
  </section>;
}

function OutcomeSummary({text}:{text:string}){
  return <>{text.split(/(\*\*[^*]+\*\*)/g).map((part,index)=>part.startsWith('**')&&part.endsWith('**')&&part.length>4
    ?<strong key={index}>{part.slice(2,-2)}</strong>:part)}</>;
}
