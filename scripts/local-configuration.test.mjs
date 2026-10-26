import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount,listenerPicker} from '../internal/modules/local_ports/assets/module.js';
class Node{
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;this.value='';}
 get childElementCount(){return this.children.length;}
 append(...nodes){for(const n of nodes){n.remove();n.parentElement=this;this.children.push(n);}}
 replaceChildren(...nodes){for(const c of this.children)c.parentElement=null;this.children=[];this.append(...nodes);}
 setAttribute(k,v){this.attrs[k]=String(v);}getAttribute(k){return this.attrs[k];}
 addEventListener(k,f){this.events.set(k,[...(this.events.get(k)||[]),f]);}removeEventListener(k,f){this.events.set(k,(this.events.get(k)||[]).filter(n=>n!==f));}
 dispatchEvent(e){for(const f of this.events.get(e.type)||[])f(e);if(e.bubbles)this.parentElement?.dispatchEvent(e);return true;}
 focus(){document.activeElement=this;}remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(n=>n!==this);this.parentElement=null;}
 querySelectorAll(){return [];}contains(n){return this===n||this.children.some(c=>c.contains(n));}
}
const tree=n=>[n,...n.children.flatMap(tree)],tick=()=>new Promise(r=>setImmediate(r));
function fire(n,type,extra={}){assert.ok(n,`missing ${type} target`);n.dispatchEvent({type,preventDefault(){},stopPropagation(){},...extra});}
function harness(data={}){
 globalThis.document={body:new Node('body'),activeElement:null,createElement:t=>new Node(t),createElementNS:(_,t)=>new Node(t),addEventListener(){},removeEventListener(){},querySelectorAll:()=>[]};globalThis.window={innerWidth:390,innerHeight:800,addEventListener(){}};
 const panel=new Node('section'),toolbar=new Node('div'),controller=new AbortController(),requests=[];let state;
 const update=(data,revision=1)=>{state={revision,port:7070,routes:[],data:{discovery_enabled:true,ignored_ports:[],ignored_processes:[],...data}};};update(data);
 const ctx={panel,toolbar,moduleId:'local_ports',signal:controller.signal,ui:{...ui,state:()=>({...state,states:{local_ports:state}})},api:async(path,options={})=>{requests.push({path,method:options.method,body:options.body&&JSON.parse(options.body)});return {listeners:[],discovery_enabled:state.data.discovery_enabled};},mutate:work=>work()};
 const view=mount(ctx);view.render(state);const all=()=>tree(panel),find=t=>all().find(n=>n.textContent===t),input=label=>{const caption=all().find(n=>n.tag==='label'&&n.textContent===label);return all().find(n=>n.tag==='input'&&n.id===caption?.htmlFor);};
 return {ctx,view,requests,panel,find,input,all,render(data={},revision=state.revision){update({...state.data,...data},revision);view.render(state);},form:()=>all().find(n=>n.tag==='form')};
}
function edit(h,label,value){const n=h.input(label);assert.ok(n,label);if(n.type==='checkbox')n.checked=value;else n.value=value;fire(n,'input',{bubbles:true});return n;}
test('Local Ports settings preserve draft nodes, focus and selection through polling',()=>{
 const h=harness();h.view.openSettings();const ports=edit(h,'Ignored ports','22, 8000-8099');ports.selectionStart=3;ports.focus();h.render({},2);h.view.openSettings();assert.equal(h.input('Ignored ports'),ports);assert.equal(ports.value,'22, 8000-8099');assert.equal(ports.selectionStart,3);assert.equal(document.activeElement,ports);h.view.dispose();
});
test('Local Ports saves typed lists and retains filters while detection is off',async()=>{
 const h=harness();h.view.openSettings();edit(h,'Automatic port detection',false);edit(h,'Ignored ports','22, 8000-8099');edit(h,'Ignored processes','sshd, postgres');
 h.ctx.mutate=async work=>{await work();h.render({discovery_enabled:false,ignored_ports:['22','8000-8099'],ignored_processes:['sshd','postgres']},2);};fire(h.form(),'submit');await tick();assert.deepEqual(h.requests[0],{path:'/api/modules/local_ports/settings',method:'PUT',body:{revision:1,discovery_enabled:false,ignored_ports:['22','8000-8099'],ignored_processes:['sshd','postgres']}});assert.equal(h.find('Save').disabled,true);assert.equal(h.input('Ignored ports').value,'22, 8000-8099');h.view.dispose();
});
test('Local Ports conflicts keep the original draft for explicit review and retry',async()=>{
 const h=harness();h.view.openSettings();edit(h,'Ignored ports','9999');h.render({ignored_ports:['22']},2);h.ctx.mutate=async work=>{await work();throw Object.assign(Error('Conflict'),{status:409});};fire(h.form(),'submit');await tick();assert.equal(h.requests[0].body.revision,1);assert.equal(h.input('Ignored ports').value,'9999');assert.ok(h.all().some(n=>/Changed elsewhere.*draft is kept/.test(n.textContent)));fire(h.find('Discard'),'click');assert.equal(h.input('Ignored ports').value,'22');h.view.dispose();
});
test('disabled picker skips scanning and keeps manual input available',async()=>{
 const h=harness({discovery_enabled:false});const port=ui.field('Service port',4567,'number');const picker=listenerPicker(h.ctx,port);await tick();assert.equal(h.requests.length,0);assert.equal(port.input.disabled,false);assert.equal(port.input.value,4567);assert.ok(tree(picker.menu).some(n=>/detection is off/.test(n.textContent)));picker.dispose();h.view.dispose();
});
test('live picker settings changes abort stale scans without changing the typed port',async()=>{
 const h=harness();let resolve,signal;h.ctx.api=(_path,options)=>{signal=options.signal;return new Promise(r=>resolve=r);};const port=ui.field('Service port',4567,'number'),picker=listenerPicker(h.ctx,port);
 picker.configure({discovery_enabled:false,ignored_ports:[],ignored_processes:[]});assert.equal(signal.aborted,true);resolve({listeners:[{port:3000,processes:[]}]});await tick();assert.equal(port.input.value,4567);assert.equal(tree(picker.menu).some(n=>n.className==='listener-option'),false);assert.ok(tree(picker.menu).some(n=>/detection is off/.test(n.textContent)));picker.dispose();h.view.dispose();
});

test('saving an unrelated toggle preserves exact process names containing commas and quotes',async()=>{
 const h=harness({ignored_processes:['api,worker','name"quoted']});h.view.openSettings();edit(h,'Automatic port detection',false);fire(h.form(),'submit');await tick();assert.deepEqual(h.requests[0].body.ignored_processes,['api,worker','name"quoted']);h.view.dispose();
});
