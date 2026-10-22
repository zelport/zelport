import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {createDashboard} from '../internal/web/assets/dashboard.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.attrs={};this.events={};this.disabled=false;this.hidden=false;this.textContent='';}
 append(...nodes){for(const node of nodes){node.parentElement=this;this.children.push(node);}}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 addEventListener(name,fn){this.events[name]=fn;}
 removeEventListener(name){delete this.events[name];}
 focus(){document.activeElement=this;}
 remove(){this.removed=true;}
 showModal(){this.open=true;}
 close(){if(!this.open)return;this.open=false;this.events.close?.();}
 querySelectorAll(){return [];}
}

function setup(){
 globalThis.document={body:new Node('body'),activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag)};
}

test('service location dialog offers the two approved choices and restores focus',()=>{
 setup();const trigger=new Node('button');let local=0,remote=0;
 const dialog=ui.serviceLocationDialog({trigger,options:[
  {label:'This computer',available:true,run:()=>local++},
  {label:'Development server',available:false,run:()=>remote++}
 ]});
 assert.equal(dialog.getAttribute('aria-labelledby')?.startsWith('dialog-title-'),true);
 assert.equal(dialog.children[0].children[0].textContent,'Where does the service run?');
 const choices=dialog.children[1].children;
 assert.deepEqual(choices.map(choice=>choice.children[0].textContent),['This computer','Development server']);
 assert.equal(choices[1].children[0].disabled,true);
 const click={preventDefault(){}};
 choices[1].children[0].events.click(click);assert.equal(remote,0);
 choices[0].children[0].events.click(click);assert.equal(local,1);assert.equal(dialog.open,false);
 assert.equal(document.activeElement,trigger);
});

test('shell maps service locations to their module forms and disables unavailable modules',()=>{
 let spec,local=0,remote=0;const navigated=[],state={module_availability:{local_ports:{enabled:true},ports_over_ssh:{enabled:false}}},modules=new Map([
  ['local_ports',{descriptor:{id:'local_ports',tab_id:'local_ports'},instance:{edit:()=>local++}}],
  ['ports_over_ssh',{descriptor:{id:'ports_over_ssh',tab_id:'ports_over_ssh'},instance:{edit:()=>remote++}}]
 ]),dashboard=createDashboard({root:new Node(),modules,client:{state:()=>state},navigate:id=>navigated.push(id),ui:{serviceLocationDialog:value=>{spec=value;return value}}});
 dashboard.openAddService({focus(){}});
 assert.equal(JSON.stringify(spec.options.map(option=>[option.label,option.available])),JSON.stringify([['This computer',true],['Development server',false]]));
 spec.options[0].run();assert.equal(local,1);assert.deepEqual(navigated,['local_ports']);
 spec.options[1].run();assert.equal(remote,0);
});

test('dashboard contains one unified Add service action',()=>{
 const html=readFileSync(new URL('../internal/web/assets/index.html',import.meta.url),'utf8');
 assert.equal((html.match(/id="add-service"/g)||[]).length,1);
 assert.match(html,/id="add-service"[^>]*>Add service<\/button>/);
});
