import {APIError} from '../api/request';
import type {Schema} from '../console/components';

export function confirmed(value:Schema['FactoryResult']) {
 const uuid=(v:unknown)=>typeof v==='string'&&/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(v);
 const version=(v:Schema['CatalogVersion'])=>!!v&&uuid(v.id)&&uuid(v.object_id)&&Number.isInteger(v.number)&&v.number>0;
 if(!value||!Array.isArray(value.agents)||!value.agents.length||value.agents.some(v=>!version(v))||(value.workflow&&!version(value.workflow)))throw new APIError('创建回执不完整，结果尚未确认。请重试同一候选或刷新对话。');
 return value;
}
