import {cleanup,render,screen} from '@testing-library/react';
import {OutcomeDetails} from './outcome';
import type {Schema} from '../console/components';

afterEach(cleanup);

function outcome(summary:string):Schema['BusinessOutcome']{
  return {state:'accepted',nodes:[{node_id:'acceptance',state:'accepted',summary}]};
}

test('最终结论中的范围说明加粗，普通结论保持原文',()=>{
  render(<OutcomeDetails outcome={outcome('本轮验收通过。**范围说明：未开展真人采纳验证，不影响本轮验收。**')}/>);
  expect(screen.getByText('范围说明：未开展真人采纳验证，不影响本轮验收。').tagName).toBe('STRONG');
  expect(screen.getByText('本轮验收通过。').tagName).toBe('P');
});

test('加粗内容仍按纯文本显示 HTML，未闭合标记保留',()=>{
  const view=render(<OutcomeDetails outcome={outcome('**<img src=x onerror=alert(1)>** **未闭合')}/>);
  expect(screen.getByText('<img src=x onerror=alert(1)>').tagName).toBe('STRONG');
  expect(view.container.querySelector('img')).toBeNull();
  expect(screen.getByText('**未闭合')).toBeTruthy();
});
