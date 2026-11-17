export type Audit={status?:string;proven?:boolean;redacted?:boolean;invalid_reason?:string};
const reasons:Record<string,string>={
  watermark_missing:'历史水位证明缺失',watermark_corrupt:'水位证明不完整',
  experience_revoked:'经验已撤销',experience_expired:'经验已到期',experience_missing:'经验版本缺失',experience_inactive:'经验未生效',body_erased:'经验正文已清除',
  policy_read_revoked:'运行读取授权已撤销',source_unavailable:'来源访问权限不足',source_invalidated:'来源真实性已失效',source_missing:'原始来源缺失',
  frozen_safety_blocked:'可信机器安全门已阻断；这不是普通暂停，恢复须使用重新验证的新版本',
  ancestor_invalid:'上游成果依赖已失效',ancestor_unproven:'上游成果尚未证明',ancestor_hash_mismatch:'上游成果哈希不一致',
  artifact_missing:'成果证明缺失',artifact_corrupt:'成果证明不完整',artifact_unproven:'成果证明无法核验',artifact_invalidated:'成果证明已失效',
  execution_input_missing:'缺少实际执行输入指纹',reference_corrupt:'经验引用证明不完整',injection_hash_mismatch:'实际注入哈希不一致',dependency_cycle:'依赖包含循环',traversal_limit:'依赖核验超过遍历边界',
};
export function auditVisible(state:Audit){return state.redacted===false&&['valid','invalid','unproven'].includes(state.status??'');}
export function AuditStatus({state}:{state:Audit}){
  const visible=auditVisible(state),reason=state.invalid_reason;
  const text=state.status==='invalid'?'依赖已失效：不可继续复用':visible&&state.status==='valid'&&state.proven===true?'服务已核验依赖有效（仅当前读取时）':'尚未证明依赖有效';
  return <div className="notice"><strong>{text}</strong>{!visible&&<p>访问受限或内容已脱敏：引用、匹配细节与哈希不展示</p>}
    {reason&&<p>原因：{Object.hasOwn(reasons,reason)?reasons[reason]:'未识别的原因码（原值已隐藏）'}</p>}
  </div>;
}

export function matchReason(value:string){
  if(value==='scoped_error_or_keyword_match')return '项目范围内错误特征或关键词匹配';
  if(value==='no_reliable_match')return '没有可靠匹配，不注入经验';
  return '匹配原因未登记或无法识别';
}
