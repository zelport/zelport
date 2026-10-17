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

test('socket permission errors have an actionable status and retain diagnostics separately',()=>{
 const error='docker: permission denied while trying to connect to the Docker API at unix:///var/run/docker.sock';
 const state={routes:[],data:{engines:[{engine:'docker',cli_status:'found',runtime_status:'permission_denied',endpoint:'unix:///var/run/docker.sock',error,stale:true}]}};
 const status=containers.engineStatuses(state)[0];
 assert.equal(status.runtime,'Permission denied');
 const section=containers.headerStatus(state).sections[0];
 assert.ok(section.rows.some(r=>r.label==='Access'&&/user running Zelport/.test(r.value)));
 assert.equal(section.help.command,'sudo usermod -aG docker "$USER"');
 assert.match(section.help.text,/rootful Docker on Linux/);
 assert.match(section.help.after,/sign in again and restart Zelport/);
 assert.match(section.help.note,/root-level/);
 assert.equal(section.diagnostic,error);
 assert.ok(!section.rows.some(r=>r.value===error));
 assert.equal(containers.headerStatus({...state,data:{engines:[{engine:'docker',cli_status:'found',runtime_status:'available'}]}}).sections[0].diagnostic,'');
});

test('Docker group instructions are not suggested for rootless sockets or other engines',()=>{
 for(const [engine,endpoint] of [['docker','unix:///run/user/1000/docker.sock'],['podman','unix:///var/run/docker.sock'],['nerdctl','']]){
  const state={routes:[],data:{engines:[{engine,endpoint,cli_status:'found',runtime_status:'permission_denied',error:'permission denied',stale:true}]}};
  const section=containers.headerStatus(state).sections.find(s=>s.title.startsWith(engine==='docker'?'Docker':engine==='podman'?'Podman':'nerdctl'));
  assert.ok(section.rows.some(r=>r.label==='Access'));
  assert.equal(section.help,undefined);
 }
});
