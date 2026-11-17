import type {Schema} from '../console/components';
export type PolicySettings=Schema['ExperiencePolicySettings'];
export type Draft={enabled:boolean;read_enabled:boolean;auto_publish:boolean;revoked:boolean;source_retention_authorized:boolean;values:Record<string,string>};
export type Field={path:string;label:string;kind:'text'|'list'|'number'|'nullable'|'select';options?:[string,string][]};
export const groups:{title:string;fields:Field[]}[]=[
  {title:'固定版本与授权范围',fields:[
    {path:'roles',label:'授权角色版本（每行一个）',kind:'list'},
    {path:'extractor_version',label:'提炼角色固定版本',kind:'text'},
    {path:'generator_version',label:'候选生成角色固定版本',kind:'text'},
    {path:'environments',label:'授权隔离环境 ID（每行一个）',kind:'list'},
    {path:'workflows',label:'授权工作流 ID（自动发布必填，每行一个）',kind:'list'},
    {path:'entrypoints',label:'授权入口 ID（自动发布必填，每行一个）',kind:'list'}]},
  {title:'独立预算与周期',fields:[
    {path:'learning_budget.calls',label:'学习调用额度',kind:'number'},{path:'learning_budget.tokens',label:'学习 Token 额度',kind:'number'},
    {path:'evaluation_budget.calls',label:'评测调用额度',kind:'number'},{path:'evaluation_budget.tokens',label:'评测 Token 额度',kind:'number'},
    {path:'budget_period_seconds',label:'额度周期（秒）',kind:'number'},{path:'candidate_interval_seconds',label:'候选最小间隔（秒）',kind:'number'}]},
  {title:'预登记评测门槛（不包含隐藏样本内容）',fields:[
    {path:'evaluation.suite_version',label:'冻结套件版本',kind:'text'},{path:'evaluation.protection_set',label:'保护集标识',kind:'text'},
    {path:'evaluation.holdout_set',label:'保留集标识',kind:'text'},{path:'evaluation.grouping',label:'独立样本分组方法',kind:'text'},
    {path:'evaluation.repeats',label:'每样本重复次数',kind:'number'},{path:'evaluation.minimum_samples',label:'评测最少独立样本数',kind:'number'},
    {path:'evaluation.primary_metric',label:'唯一主指标',kind:'select',options:[['acceptance','验收通过率'],['rework','返修次数'],['cost','实际调用费用'],['duration','耗时']]},
    {path:'evaluation.minimum_improvement',label:'最低改善幅度',kind:'number'},{path:'evaluation.non_inferiority',label:'非劣容差（可显式填 0）',kind:'nullable'},
    {path:'evaluation.uncertainty_method',label:'不确定性估计方法',kind:'text'},{path:'evaluation.confidence',label:'置信水平（大于 0 且小于 1）',kind:'number'},
    {path:'evaluation.max_candidates',label:'套件最多候选次数',kind:'number'},{path:'evaluation.multiple_comparison',label:'多重比较控制方法',kind:'text'},
    {path:'evaluation.resource_tolerance',label:'资源增量容差（可显式填 0）',kind:'nullable'}]},
  {title:'投放范围',fields:[
    {path:'delivery.mode',label:'投放模式',kind:'select',options:[['shadow','影子验证'],['canary','授权试用'],['serving','默认复用'],['paused','暂停新采用']]},
    {path:'delivery.task_scope',label:'任务范围（每行一个）',kind:'list'},{path:'delivery.traffic_basis_points',label:'试用流量（万分比，1–10000）',kind:'number'}]},
  {title:'观察、退化与来源保留授权',fields:[
    {path:'observation.window_seconds',label:'最小观察窗口（秒）',kind:'number'},{path:'observation.minimum_samples',label:'观察最少可比样本数',kind:'number'},
    {path:'observation.deadline_seconds',label:'观察最长期限（秒）',kind:'number'},{path:'observation.degradation_threshold',label:'退化门槛',kind:'number'},
    {path:'observation.retention_seconds',label:'最小来源证据保留期（秒）',kind:'number'}]}
];
const fields=groups.flatMap(group=>group.fields);
export function emptyDraft():Draft{
  return {enabled:false,read_enabled:false,auto_publish:false,revoked:false,source_retention_authorized:false,values:Object.fromEntries(fields.map(field=>[field.path,'']))};
}
export function readDraft(value:PolicySettings&{revision?:number}):Draft{
  const draft=emptyDraft();
  for(const field of fields){
    const part:unknown=field.path.split('.').reduce<unknown>((object,key)=>object&&typeof object==='object'&&key in object?(object as Record<string,unknown>)[key]:undefined,value);
    draft.values[field.path]=Array.isArray(part)?part.join('\n'):part==null?'':typeof part==='number'&&part===0&&field.kind==='number'?'':String(part);
  }
  return {...draft,enabled:value.enabled===true,read_enabled:value.read_enabled===true,auto_publish:value.auto_publish===true,revoked:value.revoked===true,source_retention_authorized:value.observation?.source_retention_authorized===true};
}
function choice<T extends string>(value:string,allowed:readonly T[]):T{
  const match=allowed.find(item=>item===value);
  if(match===undefined)throw new Error('策略包含未支持的指标或投放模式，请重新选择');
  return match;
}
export function serializeDraft(draft:Draft){
  const text=(path:string)=>draft.values[path]?.trim()??'';
  const list=(path:string)=>text(path).split('\n').map(value=>value.trim()).filter(Boolean);
  function number(path:string,nullable=false){
    const raw=text(path);if(!raw)return nullable?null:0;
    const value=Number(raw);if(!Number.isFinite(value))throw new Error(`${fields.find(field=>field.path===path)?.label}必须是有限数字`);
    return value;
  }
  const n=(path:string)=>number(path)??0;
  return {enabled:draft.enabled,read_enabled:draft.read_enabled,auto_publish:draft.auto_publish,revoked:draft.revoked,
    roles:list('roles'),extractor_version:text('extractor_version'),generator_version:text('generator_version'),environments:list('environments'),workflows:list('workflows'),entrypoints:list('entrypoints'),
    learning_budget:{calls:n('learning_budget.calls'),tokens:n('learning_budget.tokens')},evaluation_budget:{calls:n('evaluation_budget.calls'),tokens:n('evaluation_budget.tokens')},
    budget_period_seconds:n('budget_period_seconds'),candidate_interval_seconds:n('candidate_interval_seconds'),
    evaluation:{suite_version:text('evaluation.suite_version'),protection_set:text('evaluation.protection_set'),holdout_set:text('evaluation.holdout_set'),grouping:text('evaluation.grouping'),repeats:n('evaluation.repeats'),minimum_samples:n('evaluation.minimum_samples'),primary_metric:choice(text('evaluation.primary_metric'),['','acceptance','rework','cost','duration'] as const),minimum_improvement:n('evaluation.minimum_improvement'),non_inferiority:number('evaluation.non_inferiority',true),uncertainty_method:text('evaluation.uncertainty_method'),confidence:n('evaluation.confidence'),max_candidates:n('evaluation.max_candidates'),multiple_comparison:text('evaluation.multiple_comparison'),resource_tolerance:number('evaluation.resource_tolerance',true)},
    delivery:{mode:choice(text('delivery.mode'),['','shadow','canary','serving','paused'] as const),task_scope:list('delivery.task_scope'),traffic_basis_points:n('delivery.traffic_basis_points')},
    observation:{window_seconds:n('observation.window_seconds'),minimum_samples:n('observation.minimum_samples'),deadline_seconds:n('observation.deadline_seconds'),degradation_threshold:n('observation.degradation_threshold'),retention_seconds:n('observation.retention_seconds'),source_retention_authorized:draft.source_retention_authorized}} satisfies PolicySettings;
}
export function validateDraft(draft:Draft):string|undefined{
  const value=serializeDraft(draft);
  if(!draft.enabled&&!draft.auto_publish)return;
  if(draft.revoked||!draft.enabled)return '已撤销或未启用的策略不能自动发布';
  for(const field of fields){
    if(['workflows','entrypoints'].includes(field.path)&&!draft.auto_publish)continue;
    if(field.path.startsWith('delivery.')&&field.path!=='delivery.mode'&&['shadow','paused'].includes(value.delivery.mode))continue;
    const raw=draft.values[field.path]?.trim();if(!raw)return `请显式填写${field.label}`;
    if((field.kind==='number'&&Number(raw)<=0)||(field.kind==='nullable'&&Number(raw)<0))return `${field.label}超出允许范围`;
  }
  if(value.evaluation.confidence>=1)return '置信水平必须小于 1';
  if(value.evaluation.protection_set===value.evaluation.holdout_set)return '保护集与保留集必须不同';
  if(value.observation.deadline_seconds<value.observation.window_seconds)return '观察最长期限不能短于最小窗口';
  if(value.delivery.traffic_basis_points>10000)return '试用流量不能超过 10000';
  if(!draft.source_retention_authorized)return '启用前必须明确授权保留最小来源证据';
}
