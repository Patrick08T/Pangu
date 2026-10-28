import {useQuery} from '../api/hooks';
import {Page,QueryState,type Access,type Schema} from '../console/components';

export function Quality({access}:{access:Access}) {
  const query=useQuery<Schema['Quality']>(`/projects/${access.project}/quality`),value=query.data;
  return <Page title="质量与采纳" description="最近 90 天有运行回执的业务任务；同一任务的重跑不会重复增加任务数。" actions={<a className="button" href="#/evaluations">固定评测样本</a>}>
    <QueryState {...query} retry={query.reload}/>{value&&<>
      <p>业务任务：{value.tasks} · 运行次数：{value.runs} · 未评定任务：{value.unreviewed_tasks}</p>
      <p>首次采纳：{value.first_accepted} / {value.first_reviewed}（原始运行的首次人工评价）</p>
      <p>任务采纳：{value.accepted_tasks} / {value.reviewed_tasks}（每项任务最新已评定运行）</p>
      <p>需要修改：{value.needs_changes} · 拒绝：{value.rejected_tasks}</p>
      {!value.reviewed_tasks&&<p>暂无已评定样本</p>}
      <p className="muted">尚未评定的重跑不会抹掉先前评价；样本评测也计入任务数。统计不等同于真实试点验收。</p>
    </>}
  </Page>;
}
