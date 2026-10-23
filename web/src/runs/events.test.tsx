import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {Events} from './events';

afterEach(()=>{cleanup();vi.unstubAllGlobals();});
test('切换节点重置分页，事件消息作为纯文本展示',async()=>{
 const paths:string[]=[];
 vi.stubGlobal('fetch',vi.fn(async(path:string)=>{paths.push(path);return new Response(JSON.stringify({items:[{id:1,task_id:'task',attempt:1,time:'2026-10-23T00:00:00Z',type:'STARTED',message:'<script>危险</script>',truncated:false}],next_offset:20}));}));
 const {container}=render(<Events run="root" nodes={[{id:'node-run',node_id:'writer',task_id:'task',executor:'eino',attempt:1,status:'running',created_at:'2026-10-23T00:00:00Z'}]}/>);
 await screen.findByText('<script>危险</script>');expect(container.querySelector('script')).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'下一页'}));
 await waitFor(()=>expect(paths.at(-1)).toContain('offset=20'));
 fireEvent.change(screen.getByLabelText('事件范围'),{target:{value:'node-run'}});
 await waitFor(()=>expect(paths.at(-1)).toContain('offset=0&limit=20&node_run_id=node-run'));
});
