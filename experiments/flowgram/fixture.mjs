export const fixture={
  parameters:{type:'object',properties:{goal:{type:'string'},approved:{type:'boolean'}},required:['goal']},
  budget:{model_calls:8,search_calls:4,codex_sessions:2},
  nodes:[
    {id:'research',agent_version:'11111111-1111-4111-8111-111111111111'},
    {id:'code',agent_version:'22222222-2222-4222-8222-222222222222',depends_on:['research'],
      bindings:{goal:{from:'$input',pointer:'/goal',type:'string'},report:{from:'research',pointer:'/text',type:'string'}},
      when:{field:{from:'$input',pointer:'/approved',type:'boolean'},equals:true},approval:{timeout_seconds:600}},
  ],
  editor:{nodes:{research:{name:'产品研究 · Eino',x:80,y:100},code:{name:'代码实现 · Codex',x:460,y:100}},viewport:{x:0,y:0,zoom:1}},
};
