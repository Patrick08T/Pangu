import type {Schema} from '../console/components';

export function ExecutionPlan({plan}:{plan?:Schema['ExecutionPlanPreview']}){
  if(!plan)return null;
  return <section aria-label="执行图与任务映射"><h2>执行图与任务映射</h2>
    <p>{plan.changes.length?'批准后发布新版本，保留已完成成果续跑。以下差异由平台计算，历史运行保持不变。':'任务已映射到当前执行图，无需发布新版本。'}</p>
    <p className="muted">原工作流版本：{plan.base_version}</p>
    {plan.changes.map(change=><article key={change.node_id} className="review-item"><h3>{change.node_id} · {changeLabel(change)}</h3>
      <div className="table-wrap"><table><thead><tr><th>内容</th><th>原计划</th><th>批准后</th></tr></thead><tbody>
        <tr><th>Agent 版本</th><td>{change.before?.agent_version??'无'}</td><td>{change.after?.agent_version??'无'}</td></tr>
        <tr><th>前置依赖</th><td>{change.before?.depends_on?.join('、')||'无'}</td><td>{change.after?.depends_on?.join('、')||'无'}</td></tr>
        <tr><th>只读限制</th><td>{change.before?.read_only?'启用':'未显式启用'}</td><td>{change.after?.read_only?'启用':'未显式启用'}</td></tr>
        <tr><th>步骤任务</th><td>{change.before?.task||'沿用角色指令'}</td><td>{change.after?.task||'沿用角色指令'}</td></tr>
      </tbody></table></div><details><summary>查看节点完整配置差异</summary><p>原配置</p><pre>{JSON.stringify(change.before??null,null,2)}</pre><p>批准后配置</p><pre>{JSON.stringify(change.after??null,null,2)}</pre></details></article>)}
    <div className="table-wrap"><table><thead><tr><th>任务</th><th>执行节点</th></tr></thead><tbody>{plan.assignments.map(item=><tr key={item.task_id}><td>{item.task_id}</td><td>{item.node_id}</td></tr>)}</tbody></table></div>
  </section>;
}

function changeLabel(change:Schema['ExecutionPlanPreview']['changes'][number]){
  if(!change.before)return '新增';
  if(!change.after)return '移除';
  return '调整';
}
