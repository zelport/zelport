import test from 'node:test';
import assert from 'node:assert/strict';
import * as containers from '../internal/modules/containers/assets/module.js';

test('missing CLIs are neutral and runtimes are not claimed unavailable',()=>{
 const rows=containers.engineStatuses({routes:[],data:{engines:['docker','podman','nerdctl'].map(engine=>({engine,cli_status:'not_found'}))}});
 assert.equal(rows.length,3);
 for(const row of rows){assert.equal(row.cli,'Not found');assert.equal(row.runtime,'Not checked');assert.equal(row.tone,'neutral');}
});
test('runtime failures escalate only for configured services',()=>{
 const engine={engine:'docker',cli_status:'found',runtime_status:'unavailable',error:'offline',stale:true};
 assert.equal(containers.engineStatuses({routes:[],data:{engines:[engine]}})[0].tone,'pending');
 assert.equal(containers.engineStatuses({routes:[{editor:{container:{engine:'docker'}}}],data:{engines:[engine]}})[0].tone,'error');
});
test('duplicate socket inventories share a group without losing either route identity',()=>{
 const c={id:'id',name:'app',status:'running',ports:[]};
 const engines=[{engine:'docker',connection:'d',endpoint:'unix:///same',containers:[c]},{engine:'podman',connection:'p',endpoint:'unix:///same',containers:[c]}];
 const groups=containers.containerGroups({data:{engines}});
 assert.equal(groups.length,1);assert.equal(groups[0].aliases.length,2);
 engines[1].endpoint='unix:///other';assert.equal(containers.containerGroups({data:{engines}}).length,2);
});
