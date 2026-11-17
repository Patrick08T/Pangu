import {cleanup,renderHook,waitFor} from '@testing-library/react';
import {useVersions} from './catalog';
import type {Schema} from '../console/components';

const published:Schema['CatalogVersion']={id:'v1',object_id:'role',kind:'agent',number:1,source_revision:1,created_at:'2026-10-30',archived:false,spec:{agent:{id:'developer',executor:'codex',instruction:'已发布的开发职责'}}};
afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('同一角色的多个版本只读取一次当前名称，执行职责仍取各自固定版本',async()=>{
  const fetch=vi.fn(async(url:string)=>{
    if(url==='/api/catalog/role')return Response.json({id:'role',kind:'agent',name:'后端开发工程师',spec:{agent:{instruction:'尚未发布的职责'}}});
    return Response.json({...published,id:url.endsWith('v2')?'v2':'v1',number:url.endsWith('v2')?2:1});
  });vi.stubGlobal('fetch',fetch);
  const {result}=renderHook(()=>useVersions({nodes:[{id:'a',agent_version:'v1'},{id:'b',agent_version:'v2'},{id:'c',agent_version:'v1'}]}));
  await waitFor(()=>expect(result.current.versions.v1).toMatchObject({agentName:'后端开发工程师'}));
  expect(result.current.versions.v2).toMatchObject({agentName:'后端开发工程师',number:2,spec:published.spec});
  expect(fetch.mock.calls.filter(([url])=>url==='/api/catalog/role')).toHaveLength(1);
  expect(fetch.mock.calls.filter(([url])=>url==='/api/versions/v1')).toHaveLength(1);
});

test('名称读取失败不丢失已发布版本和真实 Agent 标识',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>url.includes('/catalog/')?Response.json({error:'暂不可用'},{status:503}):Response.json(published)));
  const {result}=renderHook(()=>useVersions({nodes:[{id:'a',agent_version:'v1'}]}));
  await waitFor(()=>expect(result.current.loading).toBe(false));
  expect(result.current.versions.v1).toEqual(published);
});
