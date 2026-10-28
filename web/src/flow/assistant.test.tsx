import {cleanup,render,screen} from '@testing-library/react';
import {AssistantSummary} from './assistant';
import type {Schema} from '../console/components';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
function version(executor:'eino'|'codex'):Schema['CatalogVersion']{
  return {id:'v',object_id:'agent',kind:'agent',number:2,source_revision:1,created_at:'2026-10-26',archived:false,spec:{agent:{id:'agent',executor,instruction:'调研产品并提出方案',connection_version:executor==='eino'?'conn':undefined,output_schema:{type:'object',properties:{report:{type:'string'}}}}}};
}
test('明确 Eino 使用 Codex 模型连接，不能误导成原生文件执行',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({id:'conn',kind:'connection',spec:{connection:{provider:'codex'}}}))));
  render(<AssistantSummary version={version('eino')}/>);
  expect(await screen.findByText('Eino · Codex 账号模型')).toBeTruthy();
  expect(screen.getByText(/由 Eino 管理工具/)).toBeTruthy();
  expect(screen.getByText('调研产品并提出方案')).toBeTruthy();
  expect(screen.getByText('report · 文本')).toBeTruthy();
});
test('共享角色读取固定模型连接，无需项目使用授权',async()=>{
  const fetch=vi.fn(async(_url:string)=>new Response(JSON.stringify({id:'conn',kind:'connection',spec:{connection:{provider:'codex'}}})));vi.stubGlobal('fetch',fetch);
  render(<AssistantSummary version={version('eino')}/>);
  expect(await screen.findByText('Eino · Codex 账号模型')).toBeTruthy();expect(fetch.mock.calls[0][0]).toBe('/api/versions/conn');
});
test('原生 Codex 显示文件能力，不查询模型连接',()=>{
  const fetch=vi.fn();vi.stubGlobal('fetch',fetch);
  render(<AssistantSummary version={version('codex')}/>);
  expect(screen.getByText(/可在项目独立工作区内修改文件/)).toBeTruthy();expect(fetch).not.toHaveBeenCalled();
});
