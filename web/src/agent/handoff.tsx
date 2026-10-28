import type {Schema} from '../console/components';

type Contract=Schema['HandoffContract'];
const fields:Record<keyof Contract,{name:string;hint:string;example:string}>={
  objective:{name:'交接目标',hint:'说明这个 Agent 负责解决的问题，以及不属于本职责的范围。',example:'例如：根据用户需求和上游调研，制定 Pangu 下一阶段产品规划；本次只输出方案，不修改代码。'},
  inputs:{name:'输入与上下文',hint:'说明执行前需要哪些输入、上游产物和资料；具体授权在任务中提供。',example:'例如：需要任务目标、目标用户、项目现状、资源约束，以及上游调研和竞品报告；仅使用本次授权资料。'},
  deliverables:{name:'交付物',hint:'说明要交付的报告、结构化结果或代码产物，以及格式要求。',example:'例如：交付一份中文产品规划，包含目标与非目标、需求优先级、阶段计划、验收指标和待确认事项。'},
  acceptance:{name:'验收标准',hint:'写出可以核对的完成条件，例如来源可追溯、字段齐全、测试通过。',example:'例如：每项建议关联用户问题并说明依据；事实引用可追溯，假设单独标注；每个阶段有交付物和可核对的完成条件。'},
  failure_handling:{name:'异常处理',hint:'说明资料不足、无法完成或验收未通过时，应返回哪些缺口和下一步。',example:'例如：资料不足或结论无法验证时，列出已完成部分、缺失信息、阻塞原因和下一步建议；不编造证据或声称全部完成。'},
};
export function handoffProblems(value:Contract|undefined,required=true){
  return (Object.keys(fields) as (keyof Contract)[]).flatMap(key=>{
    const text=value?.[key]??'',label=fields[key].name;
    if(required&&!text.trim())return [`${label}必填`];
    return new TextEncoder().encode(text).length>4096?[`${label}最多 4096 字节`]:[];
  });
}
export function HandoffFields({value,change,disabled=false}:{value:Contract|undefined;change:(value:Contract)=>void;disabled?:boolean}){
  return <><p className="notice">以下五项为发布和运行的必填项，草稿可以暂存。契约随版本传入执行；资料权限、预算和任务状态由现有运行配置控制。</p>
    {(Object.keys(fields) as (keyof Contract)[]).map(key=><label className="field" key={key}>{fields[key].name}<textarea aria-label={fields[key].name} aria-required="true" placeholder={fields[key].example} rows={3} maxLength={4096} disabled={disabled} value={value?.[key]??''} onChange={e=>change({...value,[key]:e.target.value})}/><small>{fields[key].hint} 发布必填，最多 4096 UTF-8 字节。</small></label>)}
    <p className="muted">验收标准会交给 Agent 执行；系统另行校验已声明的输出结构，实际交付质量仍需核验。</p>
  </>;
}
