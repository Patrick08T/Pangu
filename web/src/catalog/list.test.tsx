import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Catalog} from './list';
import {timestamp} from '../console/components';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});

test('工作流显示更新时间，默认可用筛选，切换归档状态重置分页',async()=>{
  const paths:string[]=[],updated='2026-10-30T14:19:57+08:00';
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>{
    paths.push(url);const archived=url.includes('archived=true');
    return Response.json({items:[{id:archived?'old':'current',name:archived?'旧测试':'正式流程',revision:1,archived,created_at:'2026-10-20',updated_at:updated}],next_offset:url.includes('offset=100')?undefined:100});
  }));
  render(<Catalog kind="workflow" access={{project:'p',role:'viewer',session:{id:'u',name:'成员',admin:false,csrf:'csrf'}}}/>);
  expect(await screen.findByRole('columnheader',{name:'最近更新时间'})).toBeTruthy();
  expect(await screen.findByText(timestamp(updated))).toBeTruthy();
  expect(paths[0]).toContain('archived=false');
  fireEvent.click(screen.getByRole('button',{name:'下一页'}));
  await waitFor(()=>expect(paths.some(p=>p.includes('offset=100'))).toBe(true));
  fireEvent.change(screen.getByLabelText('配置状态'),{target:{value:'true'}});
  expect(await screen.findByRole('link',{name:'旧测试'})).toBeTruthy();
  expect(paths.at(-1)).toContain('offset=0');
  expect(paths.at(-1)).toContain('archived=true');
});
