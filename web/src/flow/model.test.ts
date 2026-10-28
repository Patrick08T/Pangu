import {connect,removeNodes,duplicate,arrange,issues,type Flow} from './model';

const base:Flow={nodes:[{id:'a',agent_version:'v1'},{id:'b',agent_version:'v2',depends_on:['a'],bindings:{goal:{from:'a',pointer:'/text',type:'string'}}}]};
test('连接拒绝自环、重复边和循环，允许并行依赖',()=>{
  expect(()=>connect(base,'b','a')).toThrow('循环');
  expect(()=>connect(base,'a','a')).toThrow();
  expect(connect(base,'a','b')).toEqual(base);
  expect(connect({...base,nodes:[...base.nodes,{id:'c',agent_version:'v3'}]},'a','c').nodes[2].depends_on).toEqual(['a']);
});
test('删除保留失效映射供用户处理，复制不复制输出连接',()=>{
  const changed=removeNodes(base,['a']);expect(changed.nodes[0].bindings).toEqual(base.nodes[1].bindings);
  expect(issues(changed).some(item=>item.message.includes('上游'))).toBe(true);
  const copy=duplicate(base,'b','c');expect(copy.nodes[2].depends_on).toEqual(['a']);expect(base.nodes).toHaveLength(2);
});
test('自动排列保留显示名称和业务配置，画布信息往返不改变节点',()=>{
  const flow={...base,editor:{nodes:{a:{name:'调研',x:5,y:6}}}};
  const arranged=arrange(flow);expect(arranged.nodes).toEqual(base.nodes);expect(arranged.editor?.nodes?.a.name).toBe('调研');
  expect(arranged.editor!.nodes!.b.x).toBeGreaterThan(arranged.editor!.nodes!.a.x);
  expect(JSON.parse(JSON.stringify(arranged))).toEqual(arranged);
});
test('已知字段删除或类型变化后，在发布前提示映射错误',()=>{
  const flow:Flow={parameters:{type:'object',properties:{enabled:{type:'boolean'}}},nodes:[{id:'a',agent_version:'v1',bindings:{value:{from:'$input',pointer:'/gone',type:'string'}},when:{field:{from:'$input',pointer:'/enabled',type:'string'},equals:'yes'}}]};
  const errors=issues(flow).map(item=>item.message).join(' ');
  expect(errors).toContain('字段不存在');expect(errors).toContain('类型');
});
