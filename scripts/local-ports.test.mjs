import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {listenerPicker} from '../internal/modules/local_ports/assets/module.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;this.value='';}
 get childElementCount(){return this.children.length;}
 append(...nodes){for(const node of nodes){if(node.parentElement)node.parentElement.children=node.parentElement.children.filter(child=>child!==node);node.parentElement=this;this.children.push(node);}}
 prepend(...nodes){for(const node of [...nodes].reverse()){node.parentElement=this;this.children.unshift(node);}}
 replaceChildren(...nodes){for(const child of this.children)child.parentElement=null;this.children=[];this.append(...nodes);}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 addEventListener(name,fn){const handlers=this.events.get(name)||[];handlers.push(fn);this.events.set(name,handlers);}
 removeEventListener(name,fn){this.events.set(name,(this.events.get(name)||[]).filter(handler=>handler!==fn));}
 dispatchEvent(event){for(const handler of this.events.get(event.type)||[])handler(event);return true;}
 focus(){document.activeElement=this;}
 contains(node){return this===node||this.children.some(child=>child.contains(node));}
 closest(selector){for(let node=this;node;node=node.parentElement)if(selector==='dialog'&&node.tag==='dialog')return node;return null;}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(child=>child!==this);this.removed=true;}
 getBoundingClientRect(){return {top:100,bottom:124,left:100,width:24,height:24};}
 showPopover(){this.open=true;}
 hidePopover(){this.open=false;}
}

function setup(){
 const listeners=new Map();
 globalThis.document={
  body:new Node('body'),activeElement:null,
  createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),
  addEventListener:(name,fn)=>{const handlers=listeners.get(name)||[];handlers.push(fn);listeners.set(name,handlers);},
  removeEventListener:(name,fn)=>listeners.set(name,(listeners.get(name)||[]).filter(handler=>handler!==fn)),
  querySelectorAll:()=>[]
 };
 globalThis.window={innerWidth:1280,innerHeight:800,addEventListener(){}};
 return listeners;
}

function walk(node){return [node,...node.children.flatMap(walk)];}
function byClass(node,name){return walk(node).find(item=>item.className.split(' ').includes(name));}
function allByClass(node,name){return walk(node).filter(item=>item.className.split(' ').includes(name));}
function fire(node,type,extra={}){node.dispatchEvent({type,detail:1,preventDefault(){},stopPropagation(){},...extra});}
const flush=()=>new Promise(resolve=>setImmediate(resolve));

function createPicker(api){
 setup();
 const port=ui.field('Aux port',3000,'number'),controller=new AbortController();
 const picker=listenerPicker({moduleId:'local_ports',api,signal:controller.signal,ui},port);
 return {port,picker,controller};
}

test('listener picker loads lazily, caches opening, refreshes, and selects without submitting',async()=>{
 let calls=0;
 const listeners=[
  {port:8080,processes:[{pid:42,name:'web'}],listens_beyond_loopback:true},
  {port:9000,processes:[],listens_beyond_loopback:false}
 ];
 const {port,picker}=createPicker(async(path,options)=>{
  calls++;assert.equal(path,'/api/modules/local_ports/listeners');assert.equal(options.method,'GET');assert.ok(options.signal);
  return {listeners};
 });
 assert.equal(calls,0);assert.equal(port.input.value,3000);assert.equal(port.input.disabled,false);

 fire(picker.trigger,'click');await flush();
 assert.equal(calls,1);assert.equal(picker.menu.hidden,false);
 const rows=allByClass(picker.menu,'listener-option');assert.deepEqual(rows.map(row=>row.dataset.port),['8080','9000']);
 assert.ok(walk(rows[0]).some(node=>node.textContent==='web (42)'));
 assert.ok(walk(rows[1]).some(node=>node.textContent==='Unknown process'));
 assert.ok(walk(rows[0]).some(node=>node.textContent==='The port listens on non-loopback interfaces too.'));

 fire(picker.trigger,'click');fire(picker.trigger,'click');await flush();assert.equal(calls,1,'reopening a loaded list must not refetch');
 let inputEvents=0;port.input.addEventListener('input',()=>inputEvents++);fire(rows[0],'click');
 assert.equal(port.input.value,'8080');assert.equal(inputEvents,1);assert.equal(picker.menu.hidden,true);

 fire(picker.trigger,'click');const refresh=byClass(picker.menu,'listener-picker-refresh');fire(refresh,'click');await flush();
 assert.equal(calls,2);assert.equal(port.input.value,'8080');picker.dispose();
});

test('listener picker shows spinner then a non-blocking retry state',async()=>{
 let rejectRequest;
 const request=new Promise((_,reject)=>{rejectRequest=reject;});
 const {port,picker}=createPicker(()=>request);
 fire(picker.trigger,'click');
 assert.ok(byClass(picker.menu,'boot-spinner'),'loading state must use the shared spinner');
 rejectRequest(new Error('private operating-system diagnostic'));await flush();
 const message=byClass(picker.menu,'listener-picker-message');
 assert.equal(message.textContent,"Listening ports couldn't be detected. Enter the Aux port manually.");
 assert.equal(port.input.disabled,false);assert.ok(byClass(picker.menu,'listener-picker-refresh'));
 picker.dispose();
});

test('disposing a listener picker aborts discovery and ignores a late response',async()=>{
 let resolveRequest,requestSignal;
 const request=new Promise(resolve=>{resolveRequest=resolve;});
 const {picker}=createPicker((_path,options)=>{requestSignal=options.signal;return request;});
 fire(picker.trigger,'click');assert.ok(byClass(picker.menu,'boot-spinner'));
 picker.dispose();assert.equal(requestSignal.aborted,true);
 resolveRequest({listeners:[{port:7777,processes:[],listens_beyond_loopback:false}]});await flush();
 assert.equal(allByClass(picker.menu,'listener-option').length,0);
});
