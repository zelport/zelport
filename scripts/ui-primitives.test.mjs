import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
class Node {
 constructor(tag='div'){this.tag=tag;this.children=[];this.dataset={};this.attrs={};this.events={};}
 append(...nodes){for(const node of nodes){this.children.push(node);node.parentElement=this;}}
 prepend(...nodes){for(const node of [...nodes].reverse()){this.children.unshift(node);node.parentElement=this;}}
 replaceChildren(...nodes){this.children=[];this.append(...nodes);}
 setAttribute(k,v){this.attrs[k]=v;}
 getAttribute(k){return this.attrs[k];}
 addEventListener(k,fn){this.events[k]=fn;}
 removeEventListener(k){delete this.events[k];}
 contains(n){return this===n||this.children.some(c=>c.contains(n));}
 focus(){document.activeElement=this;}
 remove(){this.removed=true;}
 showModal(){this.open=true;}
 close(){this.open=false;this.events.close?.();}
 querySelectorAll(){return [];}
 get childElementCount(){return this.children.length;}
}
function setup(){
 const listeners=new Map();globalThis.document={body:new Node('body'),activeElement:null,createElement:t=>new Node(t),createElementNS:(_,t)=>new Node(t),addEventListener:(k,fn)=>listeners.set(k,fn),removeEventListener:k=>listeners.delete(k)};return listeners;
}
const key=k=>({key:k,preventDefault(){},stopPropagation(){this.stopped=true;}});
test('shared disclosure keeps pointer focus, selects the current option for keyboard, and releases outside listeners',()=>{
 const listeners=setup(),root=new Node(),trigger=new Node('button'),menu=new Node(),a=new Node('button'),hidden=new Node('button'),b=new Node('button');hidden.hidden=true;root.append(trigger,menu);menu.append(a,hidden,b);
 const control=ui.disclosureMenu(root,trigger,menu,{selected:n=>n===b});trigger.focus();trigger.events.click({detail:1,preventDefault(){}});
 assert.equal(document.activeElement,trigger);assert.equal(menu.hidden,false);assert.equal(listeners.size,4);
 trigger.events.keydown(key('ArrowDown'));assert.equal(document.activeElement,b);
 menu.events.keydown(key('ArrowUp'));assert.equal(document.activeElement,a);
 const escape=key('Escape');menu.events.keydown(escape);assert.equal(escape.stopped,true);assert.equal(document.activeElement,trigger);assert.equal(listeners.size,0);
 control.open();const outside=new Node();listeners.get('pointerdown')({target:outside});listeners.get('focusin')({target:outside});assert.equal(menu.hidden,false,'pointer focus must not move the clicked control before click');listeners.get('click')({target:outside});assert.equal(menu.hidden,true);assert.equal(listeners.size,0);
 control.open();control.dispose();assert.equal(listeners.size,0);assert.equal(trigger.events.click,undefined);
});
test('shared dialog cleanup runs for close button, Escape and repeated closure exactly once',()=>{
 setup();const d=new Node('dialog');let count=0;
 ui.showDialog(d,{onClose:()=>count++});assert.equal(d.open,true);d.close();d.close();assert.equal(count,1);assert.equal(d.removed,true);
});
test('dialog heading supplies the accessible name and maturity uses visible text',()=>{
 setup();const d=new Node('dialog'),header=ui.dialogHeader('Edit service',d);
 assert.ok(header.children[0].id);assert.equal(d.getAttribute('aria-labelledby'),header.children[0].id);
 const badge=ui.experimentalBadge();assert.equal(badge.tag,'span');assert.equal(badge.textContent,'Experimental');assert.notEqual(badge.getAttribute('aria-hidden'),'true');
 assert.throws(()=>ui.icon('missing'),/Unknown UI icon/);
});

test('module abort closes owned dialogs and does not open new ones',()=>{
 setup();const controller=new AbortController(),d=new Node('dialog');let count=0;
 ui.showDialog(d,{signal:controller.signal,onClose:()=>count++});controller.abort();
 assert.equal(d.open,false);assert.equal(d.removed,true);assert.equal(count,1);
 const late=new Node('dialog');ui.showDialog(late,{signal:controller.signal});assert.notEqual(late.open,true);assert.equal(late.removed,true);
});

test('invalid fields reveal collapsed sections before native validation focuses them',()=>{
 setup();const d=new Node('dialog'),section=new Node('details'),input=new Node('input');
 section.tagName='DETAILS';section.open=false;section.append(input);d.append(section);ui.showDialog(d);
 d.events.invalid({target:input});assert.equal(section.open,true);
});

test('opening another disclosure closes the first one',()=>{
 setup();const a=new Node(),b=new Node(),first=ui.disclosureMenu(new Node(),new Node('button'),a),second=ui.disclosureMenu(new Node(),new Node('button'),b);
 first.open();second.open();
 assert.equal(a.hidden,true);assert.equal(b.hidden,false);second.dispose();
});
test('route edits accept unrelated revisions but reject changed and removed services',()=>{
 const original={id:'one',module:'local_ports',domain:'one.localhost',editor:{aux_port:3000}};
 const state={revision:2,states:{local_ports:{routes:[{...original,summary:{state:'configured'}},{id:'two'}]}}};
 assert.equal(ui.routeEditRevision(original,state),2);
 assert.equal(ui.routeEditRevision(null,state),2);
 assert.equal(ui.routeEditRevision({...original,id:''},state),2);
 state.states.local_ports.routes[0]={...original,editor:{aux_port:4000}};
 assert.throws(()=>ui.routeEditRevision(original,state),e=>e.currentRoute.editor.aux_port===4000&&/changed/.test(e.message));
 state.states.local_ports.routes=[];
 assert.throws(()=>ui.routeEditRevision(original,state),/removed/);
});

test('service dialogs dispose field resources exactly once when closed',()=>{
 setup();let disposed=0;const controller=new AbortController(),panel=new Node(),state={port:8080,revision:0,states:{}};
 const view=ui.serviceModule({moduleId:'local_ports',panel,signal:controller.signal,ui:{...ui,state:()=>state},api:async()=>{},mutate:async work=>work()},{
  emptyState:{kind:'not-configured',message:'No local ports yet.',actionLabel:'+ Add service'},
  fields(){return {element:new Node(),read:()=>({}),dispose(){disposed++;}};}
 });
 view.render({routes:[],data:{}});view.edit();const dialog=document.body.children.at(-1);dialog.close();dialog.close();
 assert.equal(disposed,1);
});

test('security tooltips close on scroll and resize instead of covering unrelated content',()=>{
 setup();const listeners=new Map();globalThis.window={innerWidth:390,innerHeight:640,addEventListener:(name,fn)=>listeners.set(name,fn)};
 const {root}=ui.warningIndicator('Unencrypted connection'),[trigger,tip]=root.children;
 tip.style={};tip.showPopover=()=>tip.open=true;tip.hidePopover=()=>tip.open=false;
 tip.getBoundingClientRect=()=>({height:65,width:300});trigger.getBoundingClientRect=()=>({top:300,bottom:324,left:200});
 document.querySelectorAll=()=>tip.open?[tip]:[];
 trigger.events.focus();assert.equal(tip.open,true);
 listeners.get('scroll')();assert.equal(tip.open,false);assert.equal(tip.hidden,true);
 trigger.events.click({preventDefault(){}});assert.equal(tip.open,true);
 listeners.get('resize')();assert.equal(tip.open,false);assert.equal(tip.hidden,true);
});
