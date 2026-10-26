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
 replaceChildren(...nodes){for(const child of this.children)child.parentElement=null;this.children=[];this.append(...nodes);}
 querySelector(selector){return selector.includes(':checked')?this.querySelectorAll('input').find(input=>input.type==='radio'&&input.checked)||null:this.querySelectorAll(selector)[0]||null;}
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

test('switching the source keeps the address draft and the drawer; every source stays in it',()=>{
 const state={},{calls,modules}=sourceModules(state),navigated=[],drawer={open:true};
 const dashboard=createDashboard({root:new Node(),modules,client:{state:()=>state},navigate:id=>navigated.push(id),ui});
 dashboard.openAddService({focus(){}});calls[0].options.onSource('remote_http',{domain:'my-app',container:drawer});
 assert.equal(calls[1].id,'remote_http');assert.equal(calls[1].options.domain,'my-app');assert.equal(calls[1].options.sourceId,'remote_http');assert.equal(calls[1].options.container,drawer);
 calls[1].options.onSource('ports_over_relay',{domain:'my-app',container:drawer});
 assert.equal(calls[2].id,'ports_over_relay');assert.equal(calls[2].options.container,drawer);assert.deepEqual(navigated,[]);
 calls[2].options.onSaved();assert.deepEqual(navigated,['dashboard']);
});

test('service form lists sources as radios and reports a change with the current address',()=>{
 setup();const state={port:7070,revision:1,states:{local_ports:{routes:[]}}},changes=[];
 const source=ui.serviceModule({moduleId:'local_ports',moduleName:'Local Ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:async()=>{},mutate:work=>work()},{fields:()=>({element:ui.el('div'),read:()=>({})})});
 const dialog=source.edit(null,null,{sources:[{id:'local_ports',label:'Local Ports',description:'A port on this computer'},{id:'remote_http',label:'Remote HTTP',description:'Any HTTP or HTTPS URL'}],sourceId:'local_ports',onSource:(id,draft)=>changes.push([id,draft.domain])});
 const form=dialog.children[0];assert.equal(form.children[0].children[0].textContent,'Add service');
 const radios=dialog.querySelectorAll('input').filter(input=>input.type==='radio');assert.deepEqual(radios.map(radio=>[radio.value,radio.checked]),[['local_ports',true],['remote_http',false]]);
 const address=dialog.querySelectorAll('input').find(input=>input.type!=='radio');address.value='my-app';
 radios[1].checked=true;radios[1].events.change();assert.deepEqual(changes,[['remote_http','my-app']]);assert.equal(dialog.open,true,'the drawer stays open while its content changes');source.dispose();
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




test('a source switch swaps the content of the open drawer and disposes the previous fields',()=>{
 setup();const state={port:7070,revision:1,states:{}},disposed=[];
 const make=id=>ui.serviceModule({moduleId:id,moduleName:id,signal:new AbortController().signal,ui:{...ui,state:()=>state},api:async()=>{},mutate:work=>work()},{fields:()=>({element:ui.el('div'),read:()=>({}),dispose:()=>disposed.push(id)})});
 const sources=[{id:'local_ports',label:'Local Ports',description:'This computer',icon:'laptop'},{id:'remote_http',label:'Remote HTTP',description:'Any URL',icon:'globe'}];
 const local=make('local_ports'),remote=make('remote_http');
 const drawer=local.edit(null,null,{sources,sourceId:'local_ports'});assert.equal(drawer.open,true);assert.match(drawer.className,/service-drawer/);
 const first=drawer.children[0],same=remote.edit(null,null,{sources,sourceId:'remote_http',container:drawer,domain:'my-app'});
 assert.equal(same,drawer);assert.equal(drawer.children.length,1);assert.notEqual(drawer.children[0],first);assert.deepEqual(disposed,['local_ports']);
 assert.equal(drawer.querySelectorAll('input').find(input=>input.type!=='radio').value,'my-app');
 drawer.close();assert.deepEqual(disposed,['local_ports','remote_http']);local.dispose();remote.dispose();
});

test('pending service save blocks dismissal and a late response cannot navigate a closed dialog',async()=>{
 setup();let finish,saved=0;const state={port:7070,revision:1,states:{local_ports:{routes:[]}}};
 const source=ui.serviceModule({moduleId:'local_ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:()=>new Promise(resolve=>finish=resolve),mutate:work=>work()}, {fields:()=>({element:ui.el('div'),read:()=>({aux_port:3000})})});
 const dialog=source.edit(null,null,{onSaved(){saved++;}}),form=dialog.children[0],actions=form.children.at(-1),close=form.children[0].children[1];
 const pending=form.events.submit({preventDefault(){}});
 assert.equal(actions.children[0].disabled,true,'Cancel must wait for save');assert.equal(close.disabled,true);
 let prevented=false;dialog.events.cancel({preventDefault(){prevented=true;}});assert.equal(prevented,true,'Escape must not dismiss an in-flight save');
 dialog.close();finish({});await pending;assert.equal(saved,0,'navigation can close the dialog, so a late response must not navigate again');source.dispose();
});

test('prefilled new import uses the module creation title and action',()=>{
 setup();const source=ui.serviceModule({moduleId:'ports_over_relay',signal:new AbortController().signal,ui:{...ui,state:()=>({port:7171,revision:1,states:{}})}},{createTitle:'Import service',createSubmit:'Import service',fields:()=>({element:ui.el('div'),read:()=>({})})});
 const dialog=source.edit({id:'',domain:'sample.localhost',editor:{}}),form=dialog.children[0];
 assert.equal(form.children[0].children[0].textContent,'Import service');assert.equal(form.children.at(-1).children.at(-1).textContent,'Import service');source.dispose();
});

test('Escape and a backdrop click close the drawer; a pending save keeps it open',()=>{
 setup();const state={port:7070,revision:1,states:{local_ports:{routes:[]}}};
 const source=ui.serviceModule({moduleId:'local_ports',signal:new AbortController().signal,ui:{...ui,state:()=>state},api:()=>new Promise(()=>{}),mutate:work=>work()},{fields:()=>({element:ui.el('div'),read:()=>({aux_port:3000})})});
 const outside={target:null,clientX:10,clientY:10};
 let d=source.edit();d.getBoundingClientRect=()=>({left:500,right:1000,top:0,bottom:800});outside.target=d;
 let prevented=false;d.events.cancel({preventDefault(){prevented=true;}});assert.equal(prevented,true);assert.equal(d.open,false,'Escape closes through the drawer close path');
 d=source.edit();d.getBoundingClientRect=()=>({left:500,right:1000,top:0,bottom:800});outside.target=d;
 d.events.pointerdown({...outside,target:d});d.events.click({...outside,target:d,clientX:700});assert.equal(d.open,true,'a click inside the drawer keeps it open');
 d.events.pointerdown({...outside,target:d});d.events.click({...outside,target:d});assert.equal(d.open,false,'a backdrop click closes it');
 d=source.edit();d.getBoundingClientRect=()=>({left:500,right:1000,top:0,bottom:800});
 const form=d.children[0];form.events.submit({preventDefault(){}});d.querySelector=selector=>selector.includes('aria-busy')&&form.attrs['aria-busy']==='true'?form:null;
 d.events.pointerdown({...outside,target:d});d.events.click({...outside,target:d});d.events.cancel({preventDefault(){}});assert.equal(d.open,true,'a pending save cannot be dismissed');
 source.dispose();
});
