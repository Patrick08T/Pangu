import {cleanup,fireEvent,render,screen} from '@testing-library/react';
import {useDraft,useLeaveGuard} from './history';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
function Demo(){const draft=useDraft({nodes:[]});useLeaveGuard(!!draft.value.nodes.length);return <><output>{draft.value.nodes.length}</output><button onClick={()=>draft.change({nodes:[{id:'a',agent_version:'v'}]})}>修改</button><button onClick={draft.undo}>撤销</button><button onClick={draft.redo}>重做</button></>;}
test('撤销重做恢复完整草稿，并仅在脏数据存在时拦截离开',()=>{
  const confirm=vi.fn(()=>false);vi.stubGlobal('confirm',confirm);render(<Demo/>);
  expect(window.dispatchEvent(new Event('pangu:before-navigate',{cancelable:true}))).toBe(true);
  fireEvent.click(screen.getByText('修改'));expect(screen.getByRole('status').textContent).toBe('1');
  expect(window.dispatchEvent(new Event('pangu:before-navigate',{cancelable:true}))).toBe(false);
  fireEvent.click(screen.getByText('撤销'));expect(screen.getByRole('status').textContent).toBe('0');
  expect(window.dispatchEvent(new Event('pangu:before-navigate',{cancelable:true}))).toBe(true);
  fireEvent.click(screen.getByText('重做'));expect(screen.getByRole('status').textContent).toBe('1');expect(confirm).toHaveBeenCalledTimes(1);
});
