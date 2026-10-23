import { dagLayers } from './workflow';

test('只读 DAG 保持依赖层次，平行节点同层', () => {
  const nodes=[{id:'research',depends_on:[]},{id:'competition',depends_on:['research']},{id:'planning',depends_on:['research']},{id:'design',depends_on:['competition','planning']}];
  expect(dagLayers(nodes).map(layer=>layer.map(node=>node.id))).toEqual([['research'],['competition','planning'],['design']]);
});

test('损坏的依赖明确失败，不死循环或伪造节点', () => {
  expect(()=>dagLayers([{id:'a',depends_on:['b']},{id:'b',depends_on:['a']}])).toThrow('循环');
  expect(()=>dagLayers([{id:'a',depends_on:['missing']}])).toThrow('不存在');
});
