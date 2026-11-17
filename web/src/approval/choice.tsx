import type {Schema} from '../console/components';
type Item=Schema['ReviewItem'];
type Answer=Schema['ReviewAnswer'];

export function changesPlan(item:Item,ids:string[]=[]){
  const planned=item.planned_option_ids??[];
  return ids.length>0&&(ids.length!==planned.length||ids.some(id=>!planned.includes(id)));
}

export function validAnswer(item:Item,answer:Answer){
  const ids=answer.selected_option_ids??[],options=item.options??[];
  if(new Set(ids).size!==ids.length||ids.some(id=>!options.some(o=>o.id===id))||item.selection_mode==='single'&&ids.length>1)return false;
  if(answer.decision==='accept')return !answer.correction.trim()&&!changesPlan(item,ids)&&(!options.length||ids.length>0);
  return !!answer.correction.trim()||answer.decision==='revise'&&changesPlan(item,ids);
}

export function changeAnswer(item:Item,previous:Answer|undefined,patch:Partial<Answer>):Answer|undefined {
  const answer={...(previous??{item_id:item.id,decision:'revise' as const,correction:''}),...patch};
  if(item.options?.length){
    if(!answer.selected_option_ids?.length&&!answer.correction.trim())return undefined;
    answer.decision=answer.correction.trim()||changesPlan(item,answer.selected_option_ids)?'revise':'accept';
  }else if(answer.correction.trim()&&answer.decision==='accept')answer.decision='revise';
  return answer;
}

export function PlanOptions({item,answer,change}:{item:Item;answer?:Answer;change:(patch:Partial<Answer>)=>void}){
  const selected=answer?.selected_option_ids??[],multiple=item.selection_mode==='multiple';
  function select(id:string,checked:boolean){
    const ids=multiple?selected.filter(value=>value!==id):[];
    if(checked)ids.push(id);
    change({selected_option_ids:ids});
  }
  return <div className="review-plan-options"><p>{multiple?'可组合多选':'请选择一个方案'}，推荐项需要你明确选择。</p>
    {item.options?.map(option=><label key={option.id} className={selected.includes(option.id)?'selected':''}>
      <input type={multiple?'checkbox':'radio'} name={`plan-${item.id}`} aria-label={option.label} checked={selected.includes(option.id)} onChange={e=>select(option.id,e.target.checked)}/>
      <span><strong>{option.label}</strong>{item.recommended_option_ids?.includes(option.id)&&<span className="review-recommended">推荐</span>}{item.planned_option_ids?.includes(option.id)&&<small>当前计划</small>}<span className="review-option-description">{option.description}</span></span>
    </label>)}
    {!!selected.length&&<button type="button" onClick={()=>change({selected_option_ids:[]})}>清除方案选择</button>}
    <p className="notice">推荐理由：{item.recommendation_reason}</p>
    {changesPlan(item,selected)&&<p className="notice">此选择改变了当前计划，提交后将先修订，再请你审核。</p>}
  </div>;
}

export function ChosenOptions({item,answer}:{item:Item;answer?:Answer}){
  if(!item.options?.length)return null;
  const names=(ids:string[]=[])=>(ids.map(id=>item.options!.find(option=>option.id===id)?.label??id).join('、')||'尚未选择');
  return <div><p>你的选择：{names(answer?.selected_option_ids)}</p><p className="muted">推荐方案：{names(item.recommended_option_ids)}；推荐理由：{item.recommendation_reason}</p>
    <details><summary>查看本轮各方案与取舍</summary>{item.options.map(option=><p key={option.id}><strong>{option.label}</strong>：{option.description}</p>)}</details>
  </div>;
}
