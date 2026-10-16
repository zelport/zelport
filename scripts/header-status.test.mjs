import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
function status(id){const source=readFileSync(new URL(`../internal/modules/${id}/assets/module.js`,import.meta.url),'utf8').replaceAll('export function','function');return runInNewContext(source+';typeof headerStatus === "function" ? headerStatus : undefined',{URL});}
test('relay hosting is hidden when disabled and reports enabled runtime state',()=>{
 const get=status('built_in_relay');assert.equal(typeof get,'function');assert.equal(get({data:{enabled:false,status:{error:'Previous failure',listeners:['old:8090'],clients:3}}}),null);
 const s=get({data:{enabled:true,require_access_token:false,status:{state:'Listening',clients:2,listeners:['[::]:8090']}}});assert.equal(s.tone,'ok');assert.match(s.rows.map(r=>r.value).join(' '),/\[::\]:8090/);assert.match(s.rows.map(r=>r.value).join(' '),/2/);assert.match(s.rows.map(r=>r.value).join(' '),/Not required/);
 assert.equal(get({data:{enabled:true,status:{state:'Error',error:'Bind failed',listeners:[],clients:0}}}).tone,'error');
});
test('client distinguishes connected, connecting and failed connections without exposing URL credentials',()=>{
 const get=status('ports_over_relay');assert.equal(typeof get,'function');
 const disconnected=get({data:{enabled:false,status:{error:'Previous failure'}}});assert.equal(disconnected.status,'Disconnected');assert.equal(disconnected.tone,'neutral');assert.doesNotMatch(disconnected.rows.map(r=>r.value).join(' '),/Previous failure|reported an error/);
 const data={enabled:true,relay_url:'wss://secret:password@relay.example.com:8090/path?token=private',status:{connected:true}};
 const s=get({data});assert.equal(s.tone,'ok');assert.match(s.rows.map(r=>r.value).join(' '),/relay.example.com:8090/);assert.doesNotMatch(s.rows.map(r=>r.value).join(' '),/secret|password|private/);
 assert.equal(get({data:{...data,status:{}}}).tone,'pending');assert.equal(get({data:{...data,status:{error:'Unavailable'}}}).tone,'error');
});
