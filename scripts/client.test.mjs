import test from 'node:test';
import assert from 'node:assert/strict';
import {createClient} from '../internal/web/assets/client.js';

const response=(status,data)=>({ok:status>=200&&status<300,status,async text(){return JSON.stringify(data);}});

test('client owns state refreshes and mutation errors by originating view',async t=>{
 const requests=[],states=[],errors=[];let view='settings',next=response(200,{csrf:'token',revision:1});
 t.mock.method(globalThis,'fetch',async(path,options={})=>{requests.push({path,options});return next;});
 globalThis.document={body:{setAttribute(){},removeAttribute(){}}};
 const client=createClient({onState:state=>states.push(state),onError:(message,owner)=>errors.push([message,owner]),owner:()=>view});
 await client.refresh();assert.equal(client.state().revision,1);assert.equal(states.length,1);
 next=response(409,{error:'Conflict'});view='dashboard';
 await assert.rejects(client.mutate(()=>client.api('/api/routes',{method:'POST',body:'{}'})),/Conflict/);
 assert.deepEqual(errors,[['','dashboard'],['Conflict','dashboard']]);
 const mutation=requests.find(request=>request.path==='/api/routes');
 assert.equal(mutation.options.headers['X-CSRF-Token'],'token');assert.equal(mutation.options.headers['Content-Type'],'application/json');
 client.dispose();
});

test('client rejects paths outside the same origin API',async()=>{
 globalThis.document={body:{setAttribute(){},removeAttribute(){}}};
 const client=createClient();
 await assert.rejects(client.api('https://example.test/api/state'),/Invalid API path/);
 await assert.rejects(client.api('/app.js'),/Invalid API path/);
});

test('an unreachable linked Zelport is named and an HTML status page is never shown as the error',async t=>{
 globalThis.document={body:{setAttribute(){},removeAttribute(){}}};
 let next={ok:true,status:200,async text(){return JSON.stringify({csrf:'token',revision:1,remote_admin:{hostname:'build-server'}});}};const requests=[];
 t.mock.method(globalThis,'fetch',async(path,options={})=>{requests.push(options);return next;});
 const client=createClient();await client.refresh();
 assert.equal(requests[0].headers.Accept,'application/json','status pages answer in JSON when asked');
 next=response(503,{error:'Administration unavailable',detail:'Relay is disconnected.',status:503});
 await assert.rejects(client.api('/api/state'),{message:'build-server is unreachable. Relay is disconnected.'});
 next={ok:false,status:503,async text(){return '<!doctype html><html><title>Administration unavailable</title></html>';}};
 await assert.rejects(client.api('/api/state'),error=>!/<|doctype/i.test(error.message)&&/503/.test(error.message));
 client.dispose();
});
