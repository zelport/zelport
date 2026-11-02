import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount,sshHostPicker} from '../internal/modules/ports_over_ssh/assets/module.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;this.value='';}
 get childElementCount(){return this.children.length;}
 append(...nodes){for(const node of nodes){node.remove();node.parentElement=this;this.children.push(node);}}
 replaceChildren(...nodes){for(const child of this.children)child.parentElement=null;this.children=[];this.append(...nodes);}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 addEventListener(name,fn){const handlers=this.events.get(name)||[];handlers.push(fn);this.events.set(name,handlers);}
 removeEventListener(name,fn){this.events.set(name,(this.events.get(name)||[]).filter(handler=>handler!==fn));}
 dispatchEvent(event){for(const handler of this.events.get(event.type)||[])handler(event);if(event.bubbles)this.parentElement?.dispatchEvent(event);return true;}
 focus(){document.activeElement=this;}
 contains(node){return this===node||this.children.some(child=>child.contains(node));}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(child=>child!==this);this.parentElement=null;}
 querySelectorAll(){return [];}
}
const tree=node=>[node,...node.children.flatMap(tree)];
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function fire(node,type,extra={}){assert.ok(node,`missing ${type} target`);node.dispatchEvent({type,preventDefault(){},stopPropagation(){},...extra});}
function harness(data={}){
 globalThis.document={body:new Node('body'),activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),addEventListener(){},removeEventListener(){},querySelectorAll:()=>[]};
 globalThis.window={innerWidth:390,innerHeight:800,addEventListener(){}};
 const panel=new Node('section'),toolbar=new Node('div'),controller=new AbortController(),requests=[];
 let state={revision:1,port:7070,states:{ports_over_ssh:{routes:[]}},routes:[],data:{ssh_config_path:'',effective_config_path:'/home/test/.ssh/config',config_exists:true,connections:{},...data}};
 const ctx={panel,toolbar,moduleId:'ports_over_ssh',signal:controller.signal,ui:{...ui,state:()=>state},api:async(path,options={})=>{requests.push({path,...options,body:options.body?JSON.parse(options.body):undefined});return {hosts:['example-host'],config_path:state.data.effective_config_path};},mutate:work=>work(),refresh:async()=>{}};
 const view=mount(ctx);view.render(state);view.activate?.();
 return {panel,toolbar,view,ctx,requests,nodes:()=>tree(panel),find:text=>tree(panel).find(n=>n.getAttribute('aria-label')===text)||tree(panel).find(n=>n.textContent===text),action:text=>tree(toolbar).find(n=>n.textContent===text||n.getAttribute('aria-label')===text),render(data={},revision=state.revision,global={}){state={...state,...global,revision,data:{...state.data,...data}};view.render(state);},input(){return this.nodes().find(n=>n.tag==='input');},form(){return this.nodes().find(n=>n.tag==='form');}};
}

test('SSH configuration navigation retains a draft, input node, focus and selection through polling',()=>{
 const h=harness();assert.equal(typeof h.view.openSettings,'function');h.view.openSettings();
 const input=h.input();assert.equal(input.value,'');assert.equal(input.placeholder,'~/.ssh/config');assert.equal(document.activeElement,input);
 input.value='/tmp/draft config';input.selectionStart=4;fire(input,'input',{bubbles:true});
 h.view.deactivate();h.render({},2);h.view.activate();h.view.openSettings();
 assert.equal(h.input(),input);assert.equal(input.value,'/tmp/draft config');assert.equal(input.selectionStart,4);assert.equal(document.activeElement,input);
 h.view.dispose();assert.equal(h.toolbar.children.length,0);
});

test('settings save sends raw path and current unchanged-settings revision, then synchronizes saved values',async()=>{
 const h=harness();h.view.openSettings();const input=h.input();input.value='/tmp/team ssh';fire(input,'input',{bubbles:true});h.render({},4);
 h.ctx.mutate=async work=>{const result=await work();h.render({ssh_config_path:'/tmp/team ssh',effective_config_path:'/tmp/team ssh'},5);return result;};
 fire(h.form(),'submit');await tick();
 assert.deepEqual(h.requests.filter(r=>r.method==='PUT')[0],{path:'/api/modules/ports_over_ssh/settings',method:'PUT',body:{revision:4,ssh_config_path:'/tmp/team ssh'}});
 assert.equal(h.find('Edit file').disabled,false);assert.equal(input.title,undefined);assert.equal(input.getAttribute('aria-describedby'),undefined,'path input must not open a tooltip over its label');
});

test('polling a changed saved path preserves a draft and stale revision conflicts require explicit retry',async()=>{
 const h=harness();h.view.openSettings();h.input().value='/tmp/draft';fire(h.input(),'input',{bubbles:true});h.render({ssh_config_path:'/tmp/elsewhere',effective_config_path:'/tmp/elsewhere'},2);
 h.ctx.mutate=async work=>{await work();throw Object.assign(Error('revision conflict'),{status:409});};
 fire(h.form(),'submit');await tick();const puts=()=>h.requests.filter(r=>r.method==='PUT');assert.equal(puts()[0].body.revision,1);assert.equal(h.input().value,'/tmp/draft');
 assert.match(h.nodes().find(n=>n.getAttribute('role')==='alert').textContent,/changed elsewhere.*draft.*Save again/i);
 h.ctx.mutate=work=>work();fire(h.form(),'submit');await tick();assert.equal(puts()[1].body.revision,2);
});

test('discard restores the latest saved path and blank can be saved as the default',async()=>{
 const h=harness({ssh_config_path:'/tmp/old'});h.view.openSettings();h.input().value='/tmp/draft';fire(h.input(),'input',{bubbles:true});h.render({ssh_config_path:'/tmp/new'},2);
 fire(h.find('Discard'),'click');assert.equal(h.input().value,'/tmp/new');h.input().value='';fire(h.input(),'input',{bubbles:true});fire(h.form(),'submit');await tick();assert.equal(h.requests.filter(r=>r.method==='PUT')[0].body.ssh_config_path,'');
});

test('saving disables the form, preserves pending drafts and surfaces errors',async()=>{
 const h=harness();h.view.openSettings();h.input().value='/tmp/draft';fire(h.input(),'input',{bubbles:true});let reject;h.ctx.api=()=>new Promise((_,r)=>reject=r);fire(h.form(),'submit');
 assert.equal(h.find('Edit file').disabled,true);assert.equal(h.input().disabled,true);h.render({ssh_config_path:'/tmp/poll'},2);assert.equal(h.input().value,'/tmp/draft');reject(Error('Permission denied'));await tick();assert.equal(h.input().disabled,false);assert.equal(h.input().value,'/tmp/draft');assert.equal(h.nodes().find(n=>n.getAttribute('role')==='alert').textContent,'Permission denied');
});

test('Refresh hosts reads the configured source and lists the hosts as chips',async()=>{
 const h=harness({ssh_config_path:'/tmp/config',effective_config_path:'/tmp/config'});h.view.openSettings();const before=h.requests.length;fire(h.find('Refresh hosts'),'click');await tick();const last=h.requests.at(-1);assert.ok(h.requests.length>before);assert.equal(last.path,'/api/modules/ports_over_ssh/hosts');assert.equal(last.method,'GET');assert.ok(h.nodes().some(n=>n.className==='ssh-host-chip'&&n.textContent==='example-host'));assert.ok(h.nodes().some(n=>n.className==='ssh-hosts-count'&&n.textContent==='1'));
});

test('SSH picker help follows the configured file; the host field filters and Use selects',async()=>{
 const h=harness();h.ctx.api=async()=>({hosts:['office-server','other-host'],config_path:'/tmp/team ssh/config'});const host=ui.field('SSH host','other-host'),picker=sshHostPicker(h.ctx,host);await tick();
 const nodes=()=>tree(host.wrap);assert.ok(nodes().some(n=>/Source: \/tmp\/team ssh\/config/.test(n.textContent)));assert.equal(nodes().some(n=>/Source: ~\//.test(n.textContent)),false);
 host.input.value='office';fire(host.input,'input');assert.deepEqual(nodes().filter(n=>n.dataset.host).map(n=>n.dataset.host),['office-server']);
 fire(nodes().find(n=>n.getAttribute('aria-label')==='Use SSH host office-server'),'click');assert.equal(host.input.value,'office-server');assert.equal(nodes().find(n=>n.dataset.host==='office-server').dataset.selected,'true');picker.dispose();
});

test('SSH file editor works remotely when this computer allows it and refreshes saved hosts after save',async()=>{
 const h=harness();h.render({},1,{remote_admin:{hostname:'remote-test'},remote_file_access:true});h.view.openSettings();const browser=h.find('Edit file');assert.equal(browser.disabled,false);
 const calls=[];h.ctx.ui.fileEditor=(_ctx,options)=>{calls.push(options);return {dispose(){}};};fire(browser,'click');assert.equal(calls[0].endpoint,'/api/modules/ports_over_ssh/config-file');assert.equal(browser.disabled,true);calls[0].onClose();await calls[0].onSaved();assert.equal(h.requests.at(-1).path,'/api/modules/ports_over_ssh/hosts');assert.ok(h.find('File saved.'));h.view.dispose();
});


 test('SSH file editor requires saved settings and opens only once',()=>{
 const h=harness();h.view.openSettings();const button=h.find('Edit file');let calls=0,options;
 h.ctx.ui.fileEditor=(_ctx,value)=>{calls++;options=value;return {dispose(){}};};
 h.input().value='/tmp/draft';fire(h.input(),'input',{bubbles:true});fire(button,'click');assert.equal(calls,0);
 fire(h.find('Discard'),'click');fire(button,'click');fire(button,'click');assert.equal(calls,1);assert.equal(h.input().disabled,true);
 options.onClose();assert.equal(button.disabled,false);assert.equal(h.input().disabled,false);h.view.dispose();
});

test('SSH file path and editor stay locked in remote administration by default',()=>{
 const h=harness();h.render({},1,{remote_admin:{hostname:'remote-test'}});h.view.openSettings();
 assert.equal(h.find('Edit file').disabled,true);assert.equal(h.input().disabled,true);
 assert.ok(h.nodes?h.nodes().some(n=>n.textContent==='Remote administration cannot change files on this computer.'):h.all().some(n=>n.textContent==='Remote administration cannot change files on this computer.'));
 h.render({},2,{remote_admin:{hostname:'remote-test'},remote_file_access:true});assert.equal(h.find('Edit file').disabled,false);assert.equal(h.input().disabled,false);h.view.dispose();
});
