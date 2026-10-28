import {useEffect,useRef,useState} from 'react';

type Brief=Record<string,unknown>;
function SourceList({label,value,change,required=false,disabled}:{label:string;value:string[];change:(items:string[])=>void;required?:boolean;disabled:boolean}) {
  const [text,setText]=useState(value.join('\n')),field=useRef<HTMLTextAreaElement>(null);
  useEffect(()=>{if(JSON.stringify(text.split('\n').map(item=>item.trim()).filter(Boolean))!==JSON.stringify(value))setText(value.join('\n'));},[value]);
  useEffect(()=>{field.current?.setCustomValidity(value.length>10?'最多填写 10 项':required&&!value.length?'请填写公开搜索词，或在下方提供授权网址、项目文件':'');},[value,required]);
  return <label className="field">{label}<textarea ref={field} aria-label={label} rows={3} required={required} value={text} disabled={disabled} onChange={e=>{
    const raw=e.target.value,items=raw.split('\n').map(item=>item.trim()).filter(Boolean);setText(raw);change(items);
    e.target.setCustomValidity(items.length>10?'最多填写 10 项':required&&!items.length?'请填写至少一项':'');
  }}/><small>每行一项，最多 10 项；空行会忽略。</small></label>;
}

export function BriefInput({value,mode,change,disabled}:{value:Brief;mode:string;change:(value:Brief)=>void;disabled:boolean}) {
  function update(name:string,next:unknown){change({...value,[name]:next});}
  const values=(name:string)=>Array.isArray(value[name])?value[name] as string[]:[];
  const list=(name:string,label:string,required=false)=><SourceList key={name} label={label} value={values(name)} change={items=>update(name,items)} required={required} disabled={disabled}/>;
  const needsSources=mode==='research'||mode==='competition',hasWeb=values('public_queries').length+values('urls').length>0,hasFiles=values('documents').length>0;
  return <><label className="field task-goal">任务目标<textarea rows={5} required disabled={disabled} maxLength={16384} placeholder="想完成什么？请描述目标和希望得到的结果。" value={typeof value.question==='string'?value.question:''} onChange={e=>{update('question',e.target.value);e.target.setCustomValidity(e.target.value.trim()?'':'请填写任务目标');}}/></label>
    {mode==='competition'&&<div className="brief-grid">{list('competitors','竞品候选',true)}{list('dimensions','比较维度',true)}</div>}
    <details className="task-options"><summary>补充背景（选填）</summary><div className="brief-grid">{Object.entries({audience:'目标用户',market:'目标市场',time_scope:'时间范围',context:'现有资料',constraints:'约束条件'}).map(([name,label])=><label className="field" key={name}>{label}<textarea rows={2} maxLength={16384} disabled={disabled} value={typeof value[name]==='string'?value[name]:''} onChange={e=>update(name,e.target.value)}/></label>)}</div><p className="muted">不了解的内容可以留空，助手会说明信息不足的地方。</p></details>
    <p className="notice">{hasWeb?'已选择联网资料':'未启用联网搜索'}；{hasFiles?'已选择项目文件':'未读取项目文件'}。{!needsSources&&!hasWeb&&!hasFiles?'当前仅根据输入生成草案，不能作为竞品调研或项目现状评估。':'调研结论必须有实际读取的资料支持。'}</p>
    <details className="task-options" open><summary>资料来源{needsSources?'（至少提供一项）':'（按任务需要填写）'}</summary><p className="muted">联网搜索复用已登录的 Codex 账号。仅发送下方公开搜索词，不会把任务目标或项目文件作为搜索词发送。仅填写竞品候选不会自动联网。</p>
      {values('competitors').length>0&&<button type="button" disabled={disabled||values('public_queries').length>0} onClick={()=>update('public_queries',values('competitors').map(name=>`${name} 官方文档`))}>用竞品名称搜索官方资料</button>}
      {list('public_queries','允许公开发送的搜索词',needsSources&&!hasWeb&&!hasFiles)}{list('urls','授权公开网址')}{list('documents','授权项目相对文件')}
      <small>项目资料读取本次运行的 Git 提交，不包含未提交修改；请填写 README.md 等实际存在的相对文件路径。</small>
      {mode!=='competition'&&<>{list('competitors','竞品候选')}{list('dimensions','比较维度')}</>}
    </details>
  </>;
}
