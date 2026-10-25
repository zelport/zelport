import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import * as ui from '../internal/web/assets/ui.js';
import {headerStatus,sourceFacts,mount} from '../internal/modules/ports_over_relay/assets/module.js';

test('Overview section headings use one vertical rhythm',async()=>{
 const css=await readFile(new URL('../internal/modules/ports_over_relay/assets/style.css',import.meta.url),'utf8');
 assert.match(css,/\.relay-services-view>\.relay-setup\{margin-bottom:var\(--space-8\)\}/);
 assert.match(css,/\.relay-services-view>h2,\.relay-catalog>h2\{margin:0 0 var\(--space-3\)\}/);
 assert.match(css,/\.relay-catalog\{margin-top:var\(--space-8\)\}/);
});

class Node {
 constructor(tag){this.tag=tag;this.children=[];this.dataset={};this.style={};this.className='';this.textContent='';this.events={};this.hidden=false;this.value='';}
 get value(){return this._value||'';}
 set value(value){this._value=this.tag==='input'?String(value).replace(/[\r\n]/g,''):this.tag==='textarea'?String(value).replace(/\r\n?/g,'\n'):String(value);}
 append(...nodes){for(const node of nodes)node.parentElement=this;this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=[];this.append(...nodes);}
 setAttribute(name,value){this[name]=String(value);}
 getAttribute(name){return this[name];}
 removeAttribute(name){delete this[name];}
 addEventListener(name,fn){const previous=this.events[name];this.events[name]=previous?event=>{previous(event);return fn(event);}:fn;}
 removeEventListener(name){delete this.events[name];}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(n=>n!==this);}
 focus(){document.activeElement=this;}
 select(){this.selected=true;}
 querySelectorAll(){return [];}
}
function tree(node){return [node,...node.children.flatMap(tree)];}
function visible(node){return !node.hidden&&(!node.parentElement||visible(node.parentElement));}
async function click(node){assert.ok(node,'expected a visible action');node.events.click({preventDefault(){}});await new Promise(resolve=>setImmediate(resolve));}
function harness(data={},overrides={},routes=[]){
 globalThis.document={activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),removeEventListener(){}};
 const controller=new AbortController(),panel=new Node('section'),toolbar=new Node('div');
 let state={revision:1,port:8080,routes,data:{relay_url:'',enabled:false,has_identity:false,has_access_token:false,publish:false,administration_peers:[],status:{connected:false,records:[]},...data}};
 const requests=[],ctx={panel,toolbar,moduleId:'ports_over_relay',signal:controller.signal,ui:{...ui,...overrides,state:()=>state},api:async(path,options)=>{requests.push({path,...JSON.parse(options.body)});return {group_token:'test-only-secret'};},mutate:work=>work()};
 const view=mount(ctx);view.render(state);view.activate();
 return {view,ctx,requests,render(data={},revision=state.revision){state={...state,revision,data:{...state.data,...data}};view.render(state);},find:(text)=>tree(panel).find(n=>n['aria-label']===text)||tree(panel).find(n=>n.textContent===text),nodes:()=>tree(panel),toolbar:(text)=>tree(toolbar).find(n=>n.textContent===text)};
}

for(const scenario of [
 {data:{enabled:false,relay_url:'',status:{error:'old error'}},label:'Not configured',tone:'neutral'},
 {data:{enabled:true,relay_url:'  ',status:{}},label:'Not configured',tone:'neutral'},
 {data:{enabled:false,relay_url:'wss://relay.example.test',status:{error:'old error'}},label:'Disabled',tone:'neutral'},
 {data:{enabled:true,relay_url:'wss://relay.example.test',status:{}},label:'Connecting',tone:'pending'},
 {data:{enabled:true,relay_url:'wss://relay.example.test',status:{connected:true}},label:'Connected',tone:'ok'},
 {data:{enabled:true,relay_url:'wss://relay.example.test',status:{error:'failed'}},label:'Connection failed',tone:'error'}
])test(`Relay header and Configuration show ${scenario.label}`,async()=>{
 const status=headerStatus({data:scenario.data});
 if(scenario.label==='Not configured')assert.equal(status,null,'an unconfigured relay has no header indicator');else{assert.equal(status.status,scenario.label);assert.equal(status.tone,scenario.tone);}
 const h=harness(scenario.data);
 assert.ok(h.nodes().some(n=>n.className.includes('status-badge')&&n.textContent===scenario.label&&n.dataset.tone===scenario.tone));
});

for(const data of [{relay_url:''},{relay_url:'wss://relay.example.test',enabled:false}])test(`Relay ${data.relay_url?'disabled':'unconfigured'} connection shows the form without an onboarding detour`,()=>{
 const h=harness(data);
 assert.equal(h.nodes().some(n=>n.tag==='button'&&visible(n)&&n.textContent==='Go to Configuration'),false);
 assert.ok(h.nodes().some(n=>n.tag==='form'&&visible(n)));
 assert.equal(h.nodes().filter(n=>visible(n)&&/No services imported/.test(n.textContent)).length,0);
});

test('openSettings focuses Relay URL and keeps an edited draft across polling',async()=>{
 const h=harness({relay_url:'wss://relay.example.test'});assert.equal(typeof h.view.openSettings,'function');h.view.openSettings();
 const url=document.activeElement;assert.equal(url.value,'wss://relay.example.test');
 const form=h.nodes().find(n=>n.tag==='form');url.value='wss://draft.example.test';form.events.input({});
 h.render({relay_url:'wss://changed.example.test'},2);h.view.openSettings();
 assert.equal(document.activeElement,url);assert.equal(url.value,'wss://draft.example.test');
});

function input(h,label){const caption=h.nodes().find(n=>n.tag==='label'&&n.textContent===label);assert.ok(caption,`missing field ${label}`);return tree(caption.parentElement).find(n=>n.tag==='input'||n.tag==='select');}

test('one visible E2E field and explicit relay access mode are available without disclosure',()=>{
 const h=harness();h.view.openSettings();
 assert.equal(h.nodes().filter(n=>n.tag==='label'&&n.textContent==='E2E secret').length,1);
 assert.ok(visible(input(h,'E2E secret')));assert.equal(input(h,'E2E secret').readOnly,false);
 assert.equal(input(h,'Relay access').value,'none');assert.ok(visible(input(h,'Relay access')));
 assert.equal(h.nodes().some(n=>n.textContent==='Access and encryption'),false);
});
test('first save requires an explicit secret instead of silently creating a group',async()=>{
 const h=harness();h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');input(h,'Relay URL').value='ws://relay.example.test';form.events.input({});
 await form.events.submit({preventDefault(){}});assert.equal(h.requests.length,0);assert.equal(document.activeElement,input(h,'E2E secret'));assert.ok(h.find('Paste a shared secret or generate a new one.'));
});
test('generation is a discardable draft and is only persisted by Save',async()=>{
 const h=harness();h.view.openSettings();await click(h.find('Generate E2E secret'));const draft=input(h,'E2E secret').value;
 assert.match(draft,/^[A-Za-z0-9]{40}$/);assert.equal(h.requests.length,0);assert.equal(h.find('Save').disabled,false);
 await click(h.find('Discard'));assert.equal(input(h,'E2E secret').value,'');assert.equal(h.find('Save').disabled,true);
 await click(h.find('Generate E2E secret'));const next=input(h,'E2E secret').value;await h.nodes().find(n=>n.tag==='form').events.submit({preventDefault(){}});
 assert.equal(h.requests.length,1);assert.equal(h.requests[0].group_token,next);assert.match(h.requests[0].path,/settings$/);
});
test('Change edits the same E2E field and Discard restores its stored state',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();const original=input(h,'E2E secret');assert.equal(original.readOnly,true);
 await click(h.find('Change E2E secret'));assert.equal(input(h,'E2E secret'),original);assert.equal(original.readOnly,false);
 original.value='replacement';h.nodes().find(n=>n.tag==='form').events.input({});await click(h.find('Discard'));
 assert.equal(input(h,'E2E secret').readOnly,true);assert.equal(input(h,'E2E secret').value,'');assert.equal(h.requests.length,0);
});
test('revealing a stored secret never marks it as a replacement draft',async()=>{
 const h=harness({has_identity:true,relay_url:'ws://relay.example.test'});h.view.openSettings();await click(h.nodes().find(n=>n['aria-label']==='Reveal E2E secret'));
 assert.equal(input(h,'E2E secret').value,'test-only-secret');assert.equal(h.find('Save').disabled,true);
 const form=h.nodes().find(n=>n.tag==='form');input(h,'Relay URL').value='ws://other.example.test';form.events.input({});await form.events.submit({preventDefault(){}});
 assert.equal(h.requests.at(-1).group_token,'');
});
for(const leave of ['deactivate','dispose','change'])test(`late revealed secret cannot overwrite state after ${leave}`,async()=>{
 const h=harness({has_identity:true});h.view.openSettings();let finish;h.ctx.api=()=>new Promise(resolve=>finish=resolve);
 await click(h.nodes().find(n=>n['aria-label']==='Reveal E2E secret'));const secret=input(h,'E2E secret');
 if(leave==='change'){await click(h.find('Change E2E secret'));secret.value='replacement';}else h.view[leave]();
 finish({group_token:'late-secret'});await new Promise(resolve=>setImmediate(resolve));assert.equal(secret.value,leave==='change'?'replacement':'');
});
test('replacement drafts retain their original revision across external credential changes',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();await click(h.find('Change E2E secret'));input(h,'E2E secret').value='replacement-secret-123456789012345';const form=h.nodes().find(n=>n.tag==='form');form.events.input({});
 h.render({},2);await form.events.submit({preventDefault(){}});assert.equal(h.requests.at(-1).revision,1);
});
test('connection failure is actionable in Overview and raw diagnostics remain collapsed',()=>{
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',status:{connected:false,error:'failed to get reader: failed to read frame header: EOF',records:[]}});
 assert.ok(h.nodes().some(n=>visible(n)&&/Check.*access mode.*token/i.test(n.textContent)));
 const details=h.nodes().find(n=>n.tag==='details'&&tree(n).some(x=>x.textContent.includes('frame header')));assert.ok(details);assert.ok(!details.open);
});
test('connected relay reports zero devices without claiming an E2E mismatch',()=>{
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',status:{connected:true,records:[]}});
 assert.ok(h.nodes().some(n=>visible(n)&&n.textContent==='0 devices'));
 assert.ok(h.nodes().some(n=>visible(n)&&/same E2E secret/.test(n.textContent)));
 assert.equal(h.nodes().some(n=>/wrong E2E|invalid E2E/i.test(n.textContent)),false);
});


test('Relay configuration tracks edits, reverts, discard, and a successful save',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();let form=h.nodes().find(n=>n.tag==='form'),url=document.activeElement;
 assert.equal(h.find('Save').disabled,true);assert.equal(h.find('Discard').disabled,true);
 url.value='wss://draft.example.test';form.events.input({});assert.equal(h.find('Save').disabled,false);
 url.value='';form.events.input({});assert.equal(h.find('Save').disabled,true);
 url.value='wss://draft.example.test';form.events.input({});await click(h.find('Discard'));assert.equal(document.activeElement.value,'');assert.equal(h.find('Save').disabled,true);
 form=h.nodes().find(n=>n.tag==='form');url=document.activeElement;url.value='wss://saved.example.test';form.events.input({});
 h.ctx.mutate=async work=>{await work();h.render({relay_url:'wss://saved.example.test'},2);};
 await form.events.submit({preventDefault(){}});assert.equal(h.find('Save').disabled,true);assert.equal(document.activeElement.value,'wss://saved.example.test');
});

test('Change can be discarded before entering a replacement',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();await click(h.find('Change E2E secret'));
 assert.equal(h.find('Discard').disabled,false);await click(h.find('Discard'));
 assert.equal(input(h,'E2E secret').readOnly,true);assert.equal(h.requests.length,0);
});

test('access mode changes are explicit, discardable, and clear only on Save',async()=>{
 const h=harness({has_identity:true,has_access_token:true});h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form'),mode=input(h,'Relay access');
 mode.value='none';form.events.change({});assert.equal(input(h,'Relay access token').disabled,true);assert.equal(h.requests.length,0);
 await click(h.find('Discard'));assert.equal(mode.value,'token');assert.equal(input(h,'Relay access token').disabled,false);
 mode.value='none';form.events.change({});await form.events.submit({preventDefault(){}});assert.equal(h.requests.at(-1).clear_access_token,true);assert.equal(h.requests.at(-1).access_token,'');
});
test('token mode requires a token when none is stored',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');input(h,'Relay access').value='token';form.events.change({});
 await form.events.submit({preventDefault(){}});assert.equal(h.requests.length,0);assert.equal(document.activeElement,input(h,'Relay access token'));
});
for(const action of ['change','generate','discard','navigation','revision'])test(`late clipboard failure cannot restore a secret after ${action}`,async()=>{
 const h=harness({has_identity:true});h.view.openSettings();let reject;
 Object.defineProperty(globalThis,'navigator',{configurable:true,value:{clipboard:{writeText:()=>new Promise((_,r)=>reject=r)}}});
 await click(h.nodes().find(n=>n['aria-label']==='Copy E2E secret'));
 if(action==='change'){await click(h.find('Change E2E secret'));input(h,'E2E secret').value='replacement';h.nodes().find(n=>n.tag==='form').events.input({});}
 if(action==='generate'){await click(h.find('Change E2E secret'));await click(h.find('Generate E2E secret'));}
 const expected=action==='generate'?input(h,'E2E secret').value:action==='change'?'replacement':'';
 if(action==='discard'){await click(h.find('Change E2E secret'));await click(h.find('Discard'));}
 if(action==='navigation'){h.view.deactivate();h.view.activate();h.view.openSettings();}
 if(action==='revision')h.render({},2);
 reject(Error('Clipboard unavailable'));await new Promise(resolve=>setImmediate(resolve));
 assert.equal(input(h,'E2E secret').value,expected);assert.equal(input(h,'E2E secret').type,'password');
});
test('ordinary 40-character secret is accepted and short input fails before a request',async()=>{
 const h=harness();h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');
 input(h,'E2E secret').value='x'.repeat(31);form.events.input({});await form.events.submit({preventDefault(){}});
 assert.equal(h.requests.length,0);assert.ok(h.nodes().some(n=>/at least 32 characters/.test(n.textContent)));
 const secret='synthetic-40-character-secret-for-test-1';input(h,'E2E secret').value=secret;form.events.input({});await form.events.submit({preventDefault(){}});assert.equal(h.requests.at(-1).group_token,secret);
});
test('no-device Overview uses one service-focused empty state',()=>{
 const h=harness({has_identity:true,enabled:true,relay_url:'ws://relay.example.test',status:{connected:true,records:[]}});
 assert.equal(h.nodes().some(n=>visible(n)&&n.textContent==='Zelport administration'),false);
 assert.ok(h.nodes().some(n=>visible(n)&&n.textContent==='No other devices yet.'));
 assert.ok(h.nodes().some(n=>visible(n)&&n.textContent==='Relay server'));
});
test('remote device has one card for services and administration',()=>{
 const provider={device_id:'linux-device',group:'group',public_key:'pin',hostname:'sandbox',port:7271,publish:true,expires_at:Date.now()/1000+60,services:[{id:'service',kind:'application',domain:'sandbox.localhost'}]};
 const peer={device_id:'linux-device',hostname:'sandbox',available:true,uuid_url:'http://linux-device.zelport.localhost:7270/',hostname_url:'http://sandbox.zelport.localhost:7270/'};
 const h=harness({has_identity:true,enabled:true,relay_url:'ws://relay.example.test',administration_peers:[peer],status:{connected:true,records:[provider]}});
 const cards=h.nodes().filter(n=>n.className==='relay-provider');assert.equal(cards.length,1);
 const nodes=tree(cards[0]);assert.equal(nodes.find(n=>n.textContent==='Online').dataset.tone,'ok');assert.ok(nodes.some(n=>n.tag==='a'&&n.textContent==='Open administration'));assert.ok(nodes.some(n=>n.textContent==='sandbox.localhost:7271'));assert.equal(h.nodes().some(n=>n.textContent==='Zelport administration'),false);
 const before=cards[0];h.render({});assert.equal(h.nodes().find(n=>n.className==='relay-provider'),before);
});

test('Sources card reports connection, devices and sharing independently',()=>{
 const facts=data=>Object.fromEntries(sourceFacts({data}).map(row=>[row.label,row.value]));
 assert.deepEqual(facts({relay_url:''}),{Relay:'Not configured',Devices:'0',Sharing:'Off'});
 assert.deepEqual(facts({enabled:true,relay_url:'ws://relay.example.test',publish:true,status:{connected:true},administration_peers:[{available:true},{available:false}]}),{Relay:'Connected',Devices:'1',Sharing:'On'});
});
test('offline devices keep their card but cannot open administration or import',()=>{
 const peer={device_id:'linux',hostname:'sandbox',available:false,uuid_url:'http://linux.zelport.localhost:7270/'};
 const provider={device_id:'linux',hostname:'sandbox',group:'g',public_key:'key',publish:true,expires_at:1,services:[{kind:'application',id:'app',domain:'app.localhost'}]};
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',administration_peers:[peer],status:{connected:true,records:[provider]}});
 const card=h.nodes().find(n=>n.className==='relay-provider'),nodes=tree(card);assert.ok(nodes.some(n=>n.textContent==='Offline'));assert.equal(nodes.find(n=>n.textContent==='Open administration').tag,'span');assert.equal(nodes.find(n=>n['aria-label']==='Import service from sandbox').disabled,true);
});
test('catalog renewal metadata retains device card and import controls',()=>{
 const provider={device_id:'linux',hostname:'sandbox',group:'g',public_key:'key',publish:true,sequence:1,expires_at:Date.now()/1000+60,services:[{kind:'application',id:'app',domain:'app.localhost'}]};
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',status:{connected:true,records:[provider]}}),card=h.nodes().find(n=>n.className==='relay-provider');
 h.render({status:{connected:true,records:[{...provider,sequence:2,expires_at:provider.expires_at+60}]}});
 assert.equal(h.nodes().find(n=>n.className==='relay-provider'),card);
});

for(const secret of [' '.repeat(32), ' '+ '😀'.repeat(30)+' ', '!'.repeat(32)])test('minimum-length secret preserves every character: '+JSON.stringify(secret),async()=>{
 const h=harness();h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');
 input(h,'E2E secret').value=secret;form.events.input({});await form.events.submit({preventDefault(){}});
 assert.equal(h.requests.length,1);assert.equal(h.requests[0].group_token,secret);
});
test('31 Unicode characters fail before a settings request',async()=>{
 const h=harness();h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');
 input(h,'E2E secret').value='😀'.repeat(31);form.events.input({});await form.events.submit({preventDefault(){}});assert.equal(h.requests.length,0);
});

test('multiline paste saves the exact secret across reveal and hide',async()=>{
 const h=harness();h.view.openSettings();const form=h.nodes().find(n=>n.tag==='form');
 const raw='x'.repeat(32)+'\r\nend\r';const field=input(h,'E2E secret');
 let prevented=false;
 field.events.paste?.({clipboardData:{getData:()=>raw},preventDefault(){prevented=true;}});
 assert.ok(prevented,'multiline paste must bypass native input sanitization');
 await click(h.nodes().find(n=>n['aria-label']==='Hide E2E secret'));
 await click(h.nodes().find(n=>n['aria-label']==='Reveal E2E secret'));
 assert.ok(h.nodes().some(n=>n.tag==='textarea'&&visible(n)));
 await form.events.submit({preventDefault(){}});assert.equal(h.requests.at(-1).group_token,raw);
});
test('revealed multiline secret copies original CRLF without DOM normalization',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();const raw='x'.repeat(32)+'\r\nend';
 h.ctx.api=async()=>({group_token:raw});let copied;
 Object.defineProperty(globalThis,'navigator',{configurable:true,value:{clipboard:{writeText:async value=>{copied=value;}}}});
 await click(h.nodes().find(n=>n['aria-label']==='Reveal E2E secret'));
 assert.ok(h.nodes().some(n=>n.tag==='textarea'&&visible(n)));
 await click(h.nodes().find(n=>n['aria-label']==='Copy E2E secret'));assert.equal(copied,raw);
});

test('manual copying a revealed multiline secret retains CRLF',async()=>{
 const h=harness({has_identity:true});h.view.openSettings();const raw='x'.repeat(32)+'\r\nend';
 h.ctx.api=async()=>({group_token:raw});await click(h.nodes().find(n=>n['aria-label']==='Reveal E2E secret'));
 const area=h.nodes().find(n=>n.tag==='textarea');area.selectionStart=0;area.selectionEnd=area.value.length;let copied;
 area.events.copy?.({clipboardData:{setData:(_,text)=>{copied=text;}},preventDefault(){}});
 assert.equal(copied,raw);
});

test('catalog import action stays in the device header and addresses follow provider port changes',()=>{
 const provider={device_id:'linux',hostname:'sandbox',port:7271,group:'g',public_key:'key',publish:true,expires_at:Date.now()/1000+60,services:[{kind:'application',id:'app',domain:'app.localhost'}]};
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',status:{connected:true,records:[provider]}});
 const add=h.nodes().find(n=>n['aria-label']==='Import service from sandbox');assert.ok(add);assert.equal(add.parentElement.className,'relay-device-actions');
 assert.equal(h.nodes().filter(n=>n.tag==='tr').some(n=>tree(n).some(c=>c.tag==='button')),false);
 assert.ok(h.find('app.localhost:7271'));
 h.render({status:{connected:true,records:[{...provider,port:8088}]}});assert.ok(h.find('app.localhost:8088'));
});

for(const count of [1,2])test(`device plus selects the intended service and proposes a free domain (${count} services)`,async()=>{
 const provider={device_id:'linux',hostname:'sandbox',port:7271,group:'g',public_key:'key',publish:true,expires_at:Date.now()/1000+60,services:Array.from({length:count},(_,i)=>({kind:'application',id:`app-${i}`,domain:`app-${i}.localhost`}))};
 const imported={id:'existing',domain:'app-0.localhost',editor:{group:'g',device_id:'linux',service_id:'app-0'}};
 const edits=[];let picker;
 const h=harness({enabled:true,relay_url:'ws://relay.example.test',status:{connected:true,records:[provider]}},{serviceModule:()=>({render(){},edit:route=>edits.push(route),dispose(){}}),showDialog(dialog,options){picker=dialog;dialog.close=()=>options.onClose();return dialog;}},[imported]);
 assert.ok(h.find('Imported'));await click(h.find('Import service from sandbox'));
 if(count===2){assert.equal(edits.length,0);await click(tree(picker).find(n=>n.tag==='button'&&n.textContent==='app-1.localhost:7271'));}
 assert.equal(edits.length,1);assert.equal(edits[0].editor.service_id,count===1?'app-0':'app-1');assert.equal(edits[0].editor.provider_key,'key');assert.equal(edits[0].domain,count===1?'app-0-2.localhost':'app-1.localhost');h.view.dispose();
});
