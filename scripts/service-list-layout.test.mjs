import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount as remoteMount} from '../internal/modules/remote_http/assets/module.js';
import {mount as sshMount,sshHostPicker} from '../internal/modules/ports_over_ssh/assets/module.js';
import {mount as relayMount} from '../internal/modules/ports_over_relay/assets/module.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;}
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
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(child=>child!==this);}
 getBoundingClientRect(){return {top:100,bottom:124,left:100,width:24,height:24};}
 querySelectorAll(){return [];}
 showPopover(){this.open=true;}
 hidePopover(){this.open=false;}
}

function setup(){
 globalThis.document={body:new Node('body'),activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),addEventListener(){},removeEventListener(){},querySelectorAll:()=>[]};
 globalThis.window={innerWidth:390,innerHeight:800,addEventListener(){}};
}
function walk(node){return [node,...node.children.flatMap(walk)];}
function byClass(node,name){return walk(node).find(item=>item.className.split(' ').includes(name));}
function fire(node,type){node.dispatchEvent({type,detail:1,preventDefault(){},stopPropagation(){}});}
function mountRoute(mount,moduleId,route,data={}){
 setup();const panel=new Node('section'),controller=new AbortController(),state={port:7070,revision:1,states:{[moduleId]:{routes:[route]}}};
 const view=mount({moduleId,panel,signal:controller.signal,api:async()=>({}),mutate:async work=>work(),ui:{...ui,state:()=>state}});
 view.render({routes:[route],data});return {panel,view};
}

for(const scenario of [
 {id:'remote_http',mount:remoteMount,target:'https://example.test/path',editor:{target_url:'https://example.test/path'},headers:['Service','Target URL','Info','Actions'],detail:'Target URL: https://example.test/path',warning:'Unencrypted upstream connection.'},
 {id:'ports_over_ssh',mount:sshMount,target:'office-server → 127.0.0.1:3000',editor:{ssh_host:'office-server',ssh_port:3000},headers:['Service','SSH target','Info','Actions'],detail:'SSH host: office-server\nRemote loopback port: 3000',warning:'SSH host unavailable.'},
 {id:'ports_over_relay',mount:relayMount,target:'device-123',editor:{group:'group',device_id:'device-123',service_id:'remote-app',provider_key:'key',rewrite_origin:false},headers:['Service','Provider','Info','Actions'],detail:'Provider device: device-123\nService ID: remote-app'}
 ]){
 test(`${scenario.id} exposes headed service rows, full target details and action menu`,()=>{
  const route={id:'one',module:scenario.id,domain:'one.localhost',editor:scenario.editor,summary:{state:'configured',target:scenario.target,detail:scenario.warning||''}};
  const {panel,view}=mountRoute(scenario.mount,scenario.id,route);
  try{
   const table=byClass(panel,'local-service-list'),header=byClass(table,'service-list-header'),row=byClass(table,'service');
   assert.equal(table.getAttribute('role'),'table');
   assert.deepEqual(header.children.map(cell=>cell.textContent),scenario.headers);
   assert.equal(row.children[0].children[0].textContent,'one.localhost:7070');
   assert.equal(row.children[1].children.at(-1).textContent,scenario.target);
   const info=byClass(row,'service-target-trigger'),tip=byClass(row,'service-target-tooltip');
   assert.ok(info);assert.equal(tip.textContent,scenario.detail);
   fire(info,'click');assert.equal(tip.open,true,'target details open on tap');
   assert.ok(byClass(row,'service-menu-trigger'));
   assert.deepEqual(byClass(row,'service-menu').children.map(item=>item.textContent),['Edit','Remove']);
   if(scenario.warning)assert.equal(byClass(row,'warning-tooltip').textContent,scenario.warning);
  }finally{view.dispose();}
 });
}

test('SSH host picker explains non-interactive keys and fills only the chosen alias',async()=>{
 setup();const host=ui.field('SSH host','manual-host'),controller=new AbortController(),paths=[];
 const picker=sshHostPicker({moduleId:'ports_over_ssh',signal:controller.signal,ui,api:async path=>{paths.push(path);return {hosts:['office-server','tv.slovan.lan']};}},host);
 try{
  await new Promise(resolve=>setImmediate(resolve));
  assert.deepEqual(paths,['/api/modules/ports_over_ssh/hosts']);
  assert.equal(byClass(picker.menu,'ssh-host-picker-title').textContent,'Configured SSH hosts');
  assert.match(byClass(picker.menu,'ssh-host-help-tooltip').textContent,/physical confirmation prompts are not supported/i);
  assert.match(byClass(picker.menu,'ssh-host-help-tooltip').textContent,/system ssh command/i);
  assert.match(byClass(picker.menu,'ssh-host-help-tooltip').textContent,/Source: ~\/\.ssh\/config/);
  assert.doesNotMatch(byClass(picker.menu,'ssh-host-help-tooltip').textContent,/Include files/);
  const option=walk(picker.menu).find(node=>node.dataset.host==='tv.slovan.lan');
  assert.ok(option);
  fire(byClass(option,'ssh-host-use'),'click');
  assert.equal(host.input.value,'tv.slovan.lan');
  assert.equal(document.activeElement,host.input);
 }finally{picker.dispose();controller.abort();}
});
