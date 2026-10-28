import {Page,isManager,type Access} from './components';

export function Settings({access}:{access:Access}) {
  const groups=[{title:'账号与项目',links:[['projects','项目与成员','选择项目，管理成员及验证命令。'],['environment','运行环境','检查执行服务、登录方式和联网搜索配置。'],['connection','模型连接','共享模型 API 或 Worker 的 Codex 账号。']]},
    {title:'进阶使用',links:[['workflow','可视化编排','拖动节点、连接助手，用表单配置步骤。'],['tokens','API Token','从 API 发起任务。'],...(isManager(access)?[['github','GitHub 任务入口','让仓库事件触发任务。']]:[])]},
    {title:'结果管理',links:[['quality','质量与采纳','查看使用反馈和质量指标。'],['evaluations','评测样本','重复验证固定任务。'],['deliveries','方案到交付','从已完成的设计报告继续实施。'],...(isManager(access)?[['audit','操作审计','查询配置与运行操作记录。']]:[])]}];
  return <Page title="设置" description="从“工厂助手”对话创建能力；已有助手可直接运行，配置与进阶功能集中在这里。">{groups.map(group=><section key={group.title}><h2>{group.title}</h2><div className="settings-grid">{group.links.map(([path,title,text])=><a className="settings-card" key={path} href={`#/${path}`}><strong>{title}</strong><span>{text}</span></a>)}</div></section>)}</Page>;
}
