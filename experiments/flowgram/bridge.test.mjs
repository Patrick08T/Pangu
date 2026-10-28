import test from 'node:test';
import assert from 'node:assert/strict';
import {fixture} from './fixture.mjs';
import {toGraph,fromGraph} from './bridge.mjs';

test('往返保留完整 Pangu 定义，包括条件、审批、映射、预算和视口',()=>{
  assert.deepEqual(fromGraph(fixture,JSON.parse(JSON.stringify(toGraph(fixture)))),fixture);
});
test('画布移动与断开连线只改变对应布局和依赖',()=>{
  const graph=toGraph(fixture);graph.nodes[0].meta.position.x=150;graph.edges=[];
  const actual=fromGraph(fixture,graph),expected=structuredClone(fixture);
  expected.editor.nodes.research.x=150;expected.nodes[1].depends_on=[];
  assert.deepEqual(actual,expected);
});
test('删除节点保留失效映射供产品校验，不默默抹掉配置',()=>{
  const graph=toGraph(fixture);graph.nodes.shift();graph.edges=[];
  const actual=fromGraph(fixture,graph);
  assert.equal(actual.nodes.length,1);assert.equal(actual.nodes[0].bindings.report.from,'research');
  assert.equal(actual.editor.nodes.research,undefined);
});
test('不接受未知节点、重复节点或悬空边',()=>{
  const graph=toGraph(fixture);graph.nodes.push({...graph.nodes[0],id:'unknown'});
  assert.throws(()=>fromGraph(fixture,graph));
  graph.nodes.pop();graph.nodes.push(graph.nodes[0]);assert.throws(()=>fromGraph(fixture,graph));
  graph.nodes.pop();graph.edges.push({sourceNodeID:'missing',targetNodeID:'code'});assert.throws(()=>fromGraph(fixture,graph));
});
