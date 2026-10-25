import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {createDashboard} from '../internal/web/assets/dashboard.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.attrs={};this.events={};this.disabled=false;this.hidden=false;this.textContent='';this.value='';}
 append(...nodes){for(const node of nodes){node.parentElement=this;this.children.push(node);}}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 addEventListener(name,fn){this.events[name]=fn;}
 removeEventListener(name){delete this.events[name];}
 focus(){document.activeElement=this;}
 remove(){this.removed=true;}
 showModal(){this.open=true;}
 close(){if(!this.open)return;this.open=false;this.events.close?.();}
 querySelectorAll(selector){const tags=selector.split(',').map(tag=>tag.toUpperCase());const walk=node=>node.children.flatMap(child=>[...(tags.includes(child.tagName)?[child]:[]),...walk(child)]);return walk(this);}
 prepend(...nodes){for(const node of [...nodes].reverse()){node.parentElement=this;this.children.unshift(node);}}
 dispatchEvent(event){this.events[event.type]?.(event);return true;}
}

function setup(){
 globalThis.document={body:new Node('body'),activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag)};
}



function sourceModules(state){
 const calls=[],modules=new Map();
 for(const [id,tab,name] of [['local_ports','local','Local Ports'],['ports_over_ssh','ssh','Ports over SSH'],['remote_http','remote','Remote HTTP'],['ports_over_relay','relay','Ports over Relay']])
  modules.set(id,{descriptor:{id,tab_id:tab,name},instance:{edit:(route,group,options)=>{calls.push({id,options});return {id};}}});
 return {calls,modules};
}

test('Add service opens one form with every available source and no separate chooser',()=>{
 const state={module_availability:{ports_over_ssh:{enabled:false}}},{calls,modules}=sourceModules(state),navigated=[];
 const dashboard=createDashboard({root:new Node(),modules,client:{state:()=>state},navigate:id=>navigated.push(id),ui});
 dashboard.openAddService({focus(){}});
 assert.equal(calls.length,1);assert.equal(calls[0].id,'local_ports');
 assert.deepEqual(calls[0].options.sources.map(source=>source.label),['Local Ports','Remote HTTP','Ports over Relay']);
 assert.equal(calls[0].options.sourceId,'local_ports');assert.deepEqual(navigated,[]);
});

test('switching the source keeps the address draft; Ports over Relay opens its source page',()=>{
 const state={},{calls,modules}=sourceModules(state),navigated=[];
 const dashboard=createDashboard({root:new Node(),modules,client:{state:()=>state},navigate:id=>navigated.push(id),ui});
 dashboard.openAddService({focus(){}});calls[0].options.onSource('remote_http',{domain:'my-app'});
 assert.equal(calls[1].id,'remote_http');assert.equal(calls[1].options.domain,'my-app');assert.equal(calls[1].options.sourceId,'remote_http');
 calls[1].options.onSource('ports_over_relay',{domain:'my-app'});assert.deepEqual(navigated,['relay']);assert.equal(calls.length,2);
 calls[1].options.onSaved();assert.deepEqual(navigated,['relay','dashboard']);
});

test('service form lists sources as radios and reports a change with the current address',()=>{
 setup();const state={port:7070,revision:1,states:{local_ports:{routes:[]}}},changes=[];
 const source=ui.serviceModule({moduleId:'local_ports',moduleName:'Local Ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:async()=>{},mutate:work=>work()},{fields:()=>({element:ui.el('div'),read:()=>({})})});
 const dialog=source.edit(null,null,{sources:[{id:'local_ports',label:'Local Ports',description:'A port on this computer'},{id:'remote_http',label:'Remote HTTP',description:'Any HTTP or HTTPS URL'}],sourceId:'local_ports',onSource:(id,draft)=>changes.push([id,draft.domain])});
 const form=dialog.children[0];assert.equal(form.children[0].children[0].textContent,'Add service');
 const radios=dialog.querySelectorAll('input').filter(input=>input.type==='radio');assert.deepEqual(radios.map(radio=>[radio.value,radio.checked]),[['local_ports',true],['remote_http',false]]);
 const address=dialog.querySelectorAll('input').find(input=>input.type!=='radio');address.value='my-app';
 radios[1].checked=true;radios[1].events.change();assert.deepEqual(changes,[['remote_http','my-app']]);assert.equal(dialog.open,false);source.dispose();
});

test('dashboard contains one unified Add service action',()=>{
 const html=readFileSync(new URL('../internal/web/assets/index.html',import.meta.url),'utf8');
 assert.equal((html.match(/id="add-service"/g)||[]).length,1);
 assert.match(html,/id="add-service"[^>]*>Add service<\/button>/);
});

test('first unnamed service starts with service-1 and increments around existing domains',()=>{
 setup();let state={port:7070,revision:1,states:{local_ports:{routes:[]}}};
 const source=ui.serviceModule({moduleId:'local_ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:async()=>{},mutate:work=>work()}, {fields:()=>({element:ui.el('div'),read:()=>({})})});
 const first=source.edit(),firstDomain=first.querySelectorAll('input')[0];assert.equal(firstDomain.value,'service-1');first.close();
 state={...state,states:{local_ports:{routes:[{domain:'service-1.localhost'}]}}};
 const second=source.edit(),secondDomain=second.querySelectorAll('input')[0];assert.equal(secondDomain.value,'service-2');second.close();source.dispose();
});




test('service form Back restores all draft fields and details without submitting',()=>{
 setup();const state={port:7070,revision:1,states:{local_ports:{routes:[]}}},drafts=[];let submitted=0;
 const source=ui.serviceModule({moduleId:'local_ports',moduleName:'Local Ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:()=>submitted++,mutate:work=>work()}, {fields(){const element=ui.el('div'),target=ui.field('Service port','3000'),enabled=ui.field('Example toggle',false,'checkbox'),details=ui.el('details');element.append(target.wrap,enabled.wrap,details);return {element,read:()=>({aux_port:target.input.value})};}});
 const first=source.edit(null,null,{onBack:draft=>drafts.push(draft)}),inputs=first.querySelectorAll('input');
 inputs[0].value='draft-app';inputs[1].value='4567';inputs[2].checked=true;first.querySelectorAll('details')[0].open=true;
 const form=first.children[0],actions=form.children.at(-1);actions.children[0].events.click({preventDefault(){}});
 assert.equal(submitted,0);assert.equal(first.open,false);assert.equal(drafts.length,1);
 const second=source.edit(null,null,{draft:drafts[0],onBack(){}}),restored=second.querySelectorAll('input');
 assert.deepEqual(restored.map(input=>input.value),['draft-app','4567','']);assert.equal(restored[2].checked,true);assert.equal(second.querySelectorAll('details')[0].open,true);
 const cancel=second.children[0].children.at(-1).children[1];cancel.events.click({preventDefault(){}});assert.equal(submitted,0);
 source.dispose();
});

test('pending service save blocks dismissal and a late response cannot navigate a closed dialog',async()=>{
 setup();let finish,saved=0;const state={port:7070,revision:1,states:{local_ports:{routes:[]}}};
 const source=ui.serviceModule({moduleId:'local_ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:()=>new Promise(resolve=>finish=resolve),mutate:work=>work()}, {fields:()=>({element:ui.el('div'),read:()=>({aux_port:3000})})});
 const dialog=source.edit(null,null,{onBack(){},onSaved(){saved++;}}),form=dialog.children[0],actions=form.children.at(-1),close=form.children[0].children[1];
 const pending=form.events.submit({preventDefault(){}});
 assert.equal(actions.children[0].disabled,true,'Back must wait for save');assert.equal(actions.children[1].disabled,true,'Cancel must wait for save');assert.equal(close.disabled,true);
 let prevented=false;dialog.events.cancel({preventDefault(){prevented=true;}});assert.equal(prevented,true,'Escape must not dismiss an in-flight save');
 dialog.close();finish({});await pending;assert.equal(saved,0,'navigation can close the dialog, so a late response must not navigate again');source.dispose();
});

test('prefilled new import uses the module creation title and action',()=>{
 setup();const source=ui.serviceModule({moduleId:'ports_over_relay',signal:new AbortController().signal,ui:{...ui,state:()=>({port:7171,revision:1,states:{}})}},{createTitle:'Import service',createSubmit:'Import',fields:()=>({element:ui.el('div'),read:()=>({})})});
 const dialog=source.edit({id:'',domain:'sample.localhost',editor:{}}),form=dialog.children[0];
 assert.equal(form.children[0].children[0].textContent,'Import service');assert.equal(form.children.at(-1).children.at(-1).textContent,'Import');source.dispose();
});
