import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import App from '../App';

afterEach(()=>{cleanup();vi.unstubAllGlobals();window.location.hash='';});
test('默认进入工厂助手，主导航包含角色与工作流，管理功能集中在设置',async()=>{
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/auth/me')?{id:'u',name:'操作者',admin:false,csrf:'csrf'}:url.endsWith('/projects')?[{id:'p',name:'项目甲'}]:url.endsWith('/auth/memberships')?[{project_id:'p',role:'operator'}]:{items:[]}))));
  render(<App/>);await screen.findByRole('heading',{name:'工厂助手'});
  expect([...screen.getByRole('navigation',{name:'主导航'}).querySelectorAll('a')].map(a=>a.textContent)).toEqual(['工厂助手','任务记录','Agent 角色','工作流','自动化']);
  expect(screen.getByRole('link',{name:'设置'}).getAttribute('href')).toBe('#/settings');
  expect(screen.queryByRole('link',{name:'API Token'})).toBeNull();
});
test('未登录只显示飞书入口，不加载业务记录',async()=>{
  const fetcher=vi.fn(async()=>new Response(JSON.stringify({error:'请登录'}),{status:401}));vi.stubGlobal('fetch',fetcher);
  render(<App/>);
  expect((await screen.findByRole('link',{name:'使用飞书登录'})).getAttribute('href')).toBe('/api/auth/login');
  expect(fetcher).toHaveBeenCalledTimes(1);expect(screen.queryByRole('link',{name:'模型连接'})).toBeNull();
});
test('查看者可查询运行，隐藏管理入口，过期会话立即移除业务页面',async()=>{
  window.location.hash='/runs';
  vi.stubGlobal('fetch',vi.fn(async(url:string)=>new Response(JSON.stringify(url.endsWith('/auth/me')?{id:'u',name:'查看者甲',admin:false,csrf:'csrf'}:url.endsWith('/projects')?[{id:'p',name:'项目甲',branch:'main',worker_id:'worker',source:''}]:url.endsWith('/auth/memberships')?[{project_id:'p',role:'viewer'}]:{items:[]}))));
  render(<App/>);await screen.findByText('当前页没有符合条件的运行。可更改筛选或继续翻页。');
  expect(screen.queryByRole('link',{name:'模型连接'})).toBeNull();expect(screen.queryByRole('link',{name:'发起运行'})).toBeNull();
  fireEvent(window,new Event('pangu:unauthenticated'));
  await screen.findByRole('link',{name:'使用飞书登录'});expect(screen.queryByText('查看者甲')).toBeNull();
});
