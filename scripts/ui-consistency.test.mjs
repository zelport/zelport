import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount} from '../internal/modules/docker_sandboxes/assets/module.js';
class Node {
 constructor(){this.children=[];this.events={};this.dataset={};}
 append(...nodes){this.children.push(...nodes);}
 setAttribute(name,value){this[name]=value;}
 addEventListener(name,fn){this.events[name]=fn;}
 focus(){}
}
globalThis.document={createElement:()=>new Node(),activeElement:null,addEventListener(){},removeEventListener(){}};
const port=(sandbox,host)=>({sandbox_port:sandbox,host_port:host,protocol:'tcp',host_ip:'127.0.0.1'});
function mapping(settings,ports){
 let chosen;
 const def=mount({panel:new Node(),ui:{...ui,serviceModule:(_,d)=>d,picker:(_,options,value)=>{chosen=value;return {wrap:new Node(),value}},field:()=>({wrap:new Node(),input:{}})}});
 def.fields(settings,null,{data:{sandboxes:[{id:'sandbox',ports}]}});return chosen;
}
test('editing an automatic sandbox mapping selects the effective existing port',()=>{
 assert.equal(mapping({sandbox_id:'sandbox',sandbox_port:3000},[port(3000,5173)]),'0');
 assert.equal(mapping({sandbox_id:'sandbox',sandbox_port:3000,selected:port(3000,7000)},[port(3000,5173),port(3000,7000)]),'1');
 assert.equal(mapping({sandbox_id:'sandbox',sandbox_port:3000},[port(8080,5173)]),'new');
});
test('Escape in an open picker prevents native dialog dismissal',()=>{
 const picker=ui.picker('Mapping',[{value:'one',label:'One'}],'one');
 const root=picker.wrap.children[1],trigger=root.children[0],menu=root.children[1];
 menu.hidden=false;let prevented=false,stopped=false;
 menu.events.keydown({key:'Escape',preventDefault(){prevented=true},stopPropagation(){stopped=true}});
 assert.equal(menu.hidden,true);assert.equal(trigger['aria-expanded'],'false');assert.equal(prevented,true);assert.equal(stopped,true);
});

test('new sandbox services do not preselect an existing or new mapping',()=>{
 assert.equal(mapping({},[port(3000,5173)]),'');
 assert.equal(mapping({},[]),'');
});
