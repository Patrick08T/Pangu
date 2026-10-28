import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {useState} from 'react';
import {AgentConfiguration} from './configuration';
import type {Draft} from './model';

afterEach(()=>{cleanup();vi.restoreAllMocks();vi.unstubAllGlobals();});
function Editor({model='',editable=true}:{model?:string;editable?:boolean}){
 const [draft,setDraft]=useState<Draft>({name:'代码助手',spec:{agent:{id:'agent',executor:'codex',instruction:'只读检查',model}}});
 return <><AgentConfiguration value={draft} change={setDraft} disabled={!editable} editable={editable}/><output>{draft.spec.agent?.model||'默认'}</output></>;
}
test('Codex 模型从下拉选择，切换保存模型 ID，默认选项清空覆盖',async()=>{
 const fetch=vi.fn(async(_url:string)=>new Response(JSON.stringify({items:[{id:'model-a',name:'模型 A'},{id:'model-b',name:'模型 B'}]})));
 vi.stubGlobal('fetch',fetch);render(<Editor/>);fireEvent.click(screen.getByRole('button',{name:'模型与执行'}));
 await screen.findByRole('option',{name:'模型 A'});expect(screen.queryByRole('textbox',{name:/Codex 模型/})).toBeNull();
 const select=screen.getByRole('combobox',{name:'Codex 模型'});fireEvent.change(select,{target:{value:'model-b'}});expect(screen.getByRole('status').textContent).toBe('model-b');
 fireEvent.change(select,{target:{value:''}});expect(screen.getByRole('status').textContent).toBe('默认');
 expect(fetch.mock.calls[0][0]).toBe('/api/models/codex');
});
test('目录读取失败不丢失已保存模型，并可重试读取',async()=>{
 const fetch=vi.fn().mockRejectedValueOnce(Error('断网')).mockResolvedValue(new Response(JSON.stringify({items:[{id:'model-c',name:'模型 C'}]})));
 vi.stubGlobal('fetch',fetch);render(<Editor model="saved-model"/>);fireEvent.click(screen.getByRole('button',{name:'模型与执行'}));
 await screen.findByRole('button',{name:'重试'});expect((screen.getByRole('combobox',{name:'Codex 模型'}) as HTMLSelectElement).value).toBe('saved-model');
 fireEvent.click(screen.getByRole('button',{name:'重试'}));await screen.findByRole('option',{name:'模型 C'});
 expect(screen.getByRole('status').textContent).toBe('saved-model');
});
test('只读成员只显示当前模型，不请求配置选择接口',async()=>{
 const fetch=vi.fn();vi.stubGlobal('fetch',fetch);render(<Editor model="saved-model" editable={false}/>);fireEvent.click(screen.getByRole('button',{name:'模型与执行'}));
 await waitFor(()=>expect(screen.getByRole('combobox',{name:'Codex 模型'}).hasAttribute('disabled')).toBe(true));expect(fetch).not.toHaveBeenCalled();
});
