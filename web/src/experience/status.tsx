type LifecycleProps={validity?:string;rollout?:string};
type Decision={outcome?:string;category?:string;decision_hash?:string};
const validityLabels:Record<string,string>={candidate:'候选：尚未验证',active:'已验证真实性',superseded:'已有替代修订',revoked:'已撤销',expired:'已过期'};
const rolloutLabels:Record<string,string>={shadow:'影子验证：不影响业务任务',canary:'授权小范围试用',serving:'默认复用',paused:'已暂停新采用',observing:'观察中：尚未成为稳定默认',stable:'稳定版本',published:'角色已发布：不等于流程已采用',adopted:'入口已采用：等待观察',rolled_back:'已回退',inconclusive:'证据不足：不能声称改善或退化'};
const outcomeLabels:Record<string,string>={passed:'通过受保护判定',accepted:'通过受保护判定',rejected:'未通过',failed:'未通过',inconclusive:'证据不足',pending:'尚无判定'};

export function Lifecycle({validity,rollout}:LifecycleProps){
  return <div className="experience-states">{validity&&<span className="status">{validityLabels[validity]??`未知状态（${validity}）`}</span>}{rollout&&<span className="status">{rolloutLabels[rollout]??`未知状态（${rollout}）`}</span>}</div>;
}

export function DecisionSummary({decision}:{decision?:Decision}){
  if(!decision)return <p>尚无受保护评测判定；不会据此自动发布。</p>;
  return <dl><dt>粗粒度判定</dt><dd>{outcomeLabels[decision.outcome??'']??'未知判定：不可据此晋级'}</dd><dt>类别</dt><dd>{decision.category||'未提供'}</dd><dt>判定摘要指纹</dt><dd className="identifier">{decision.decision_hash||'未提供'}</dd></dl>;
}

export function CapabilityNotice({available,reason}:{available?:boolean;reason?:string}){
  return <div className="notice" role="status"><strong>{available===true?'服务已报告此能力可用':'不可执行：服务尚未确认能力'}</strong>{reason&&<p>{reason}</p>}<p>接口或单元测试通过不代表真实模型收益；缺少可信环境、检查器或授权时保持关闭，不模拟成功。</p></div>;
}
