import test from 'node:test';
import assert from 'node:assert/strict';
import {sandboxStatus} from '../internal/modules/docker_sandboxes/assets/module.js';
test('CLI metadata never implies successful discovery',()=>{
 const cli={status:'found',version:'v0.48.0',path:'/test/sbx'};
 assert.equal(sandboxStatus({data:{cli,stale:true,error:'login failed'}}).status,'Discovery failed');
 assert.equal(sandboxStatus({data:{cli,stale:false}}).status,'Available');
 assert.equal(sandboxStatus({data:{cli,stale:true}}).status,'Checking');
 assert.equal(sandboxStatus({data:{cli,stale:true,updated_at:'2026-10-14T10:00:00Z'}}).status,'Partial discovery failed');
 assert.equal(sandboxStatus({data:{cli:{status:'not_found'},error:'missing'}}).status,'CLI not found');
});
test('version failure remains visible without hiding healthy inventory',()=>{
 const status=sandboxStatus({data:{cli:{status:'found',error:'version failed'},stale:false}});
 assert.equal(status.status,'Available');assert.equal(status.version,'Version unavailable');assert.equal(status.error,'version failed');
});
