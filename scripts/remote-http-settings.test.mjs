import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount} from '../internal/modules/remote_http/assets/module.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;this.value='';}
 get childElementCount(){return this.children.length;}
 append(...nodes){for(const node of nodes){node.remove();node.parentElement=this;this.children.push(node);}}
 replaceChildren(...nodes){for(const child of this.children)child.parentElement=null;this.children=[];this.append(...nodes);}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 removeAttribute(name){delete this.attrs[name];}
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
 const panel=new Node('section'),toolbar=new Node('div'),requests=[],controller=new AbortController();
 let state={revision:1,port:7070,states:{remote_http:{routes:[]}},routes:[],data:{ca_cert_path:'',effective_ca_cert_path:'',proxy_url:'',proxy_username:'',has_proxy_password:false,configuration_error:'',default_ca_cert_path:'/home/test/.config/zelport/additional-ca.pem',default_ca_cert_display_path:'~/.config/zelport/additional-ca.pem',...data}};
 const ctx={panel,toolbar,moduleId:'remote_http',signal:controller.signal,ui:{...ui,state:()=>state},api:async(path,options={})=>{requests.push({path,method:options.method,body:options.body?JSON.parse(options.body):undefined});return {};},mutate:work=>work(),refresh:async()=>{}};
 const view=mount(ctx);view.render(state);
 const all=()=>tree(panel),find=text=>all().find(n=>n.getAttribute('aria-label')===text)||all().find(n=>n.textContent===text),input=label=>{const caption=all().find(n=>n.tag==='label'&&n.textContent===label);return all().find(n=>n.tag==='input'&&n.id===caption?.htmlFor);};
 return {ctx,view,panel,toolbar,requests,all,find,input,form:()=>all().find(n=>n.tag==='form'),render(data={},revision=state.revision,global={}){state={...state,...global,revision,data:{...state.data,...data}};view.render(state);}};
}
function edit(h,label,value){const input=h.input(label);assert.ok(input,label);if(input.type==='checkbox')input.checked=value;else input.value=value;fire(input,'input',{bubbles:true});return input;}

test('Remote HTTP Configuration preserves CA, proxy and password drafts through polling and view changes',()=>{
 const h=harness({has_proxy_password:true});h.view.openSettings();edit(h,'Proxy URL','http://proxy.example.test:8080');const ca=edit(h,'Additional CA certificate file','/tmp/team.pem'),password=edit(h,'Proxy password','new-secret');ca.selectionStart=3;ca.focus();
 h.render({},2);fire(tree(h.toolbar).find(n=>n.textContent==='Overview'),'click');h.view.openSettings();
 assert.equal(h.input('Additional CA certificate file'),ca);assert.equal(ca.value,'/tmp/team.pem');assert.equal(ca.selectionStart,3);assert.equal(password.value,'new-secret');assert.equal(h.all().some(n=>n.textContent==='new-secret'),false);h.view.dispose();
});

test('Remote HTTP settings send typed credentials once and clear password inputs after successful save',async()=>{
 const h=harness();h.view.openSettings();edit(h,'Additional CA certificate file','/tmp/ca.pem');edit(h,'Proxy URL','socks5://proxy.example.test:1080');edit(h,'Proxy username','account');edit(h,'Proxy password','test-only-secret');
 h.ctx.mutate=async work=>{const result=await work();h.render({ca_cert_path:'/tmp/ca.pem',proxy_url:'socks5://proxy.example.test:1080',proxy_username:'account',has_proxy_password:true},2);return result;};
 fire(h.form(),'submit');await tick();assert.deepEqual(h.requests[0],{path:'/api/modules/remote_http/settings',method:'PUT',body:{revision:1,ca_cert_path:'/tmp/ca.pem',proxy_url:'socks5://proxy.example.test:1080',proxy_username:'account',proxy_password:'test-only-secret',clear_proxy_password:false}});
 assert.equal(h.input('Proxy password').value,'');assert.equal(h.find('Save settings').disabled,true);h.view.dispose();
});

test('Remote HTTP conflicts retain drafts and the original revision until explicit retry',async()=>{
 const h=harness();h.view.openSettings();edit(h,'Proxy URL','http://proxy.example.test:8080');h.render({},4);h.ctx.mutate=async work=>{await work();throw Object.assign(Error('Conflict'),{status:409});};
 fire(h.form(),'submit');await tick();assert.equal(h.requests[0].body.revision,1);assert.equal(h.input('Proxy URL').value,'http://proxy.example.test:8080');assert.match(h.all().find(n=>n.getAttribute('role')==='alert').textContent,/draft.*Save again/i);
 h.ctx.mutate=work=>work();fire(h.form(),'submit');await tick();assert.equal(h.requests[1].body.revision,4);h.view.dispose();
});

test('Remote HTTP discard clears entered secrets and restores the latest saved fields',()=>{
 const h=harness({proxy_url:'http://old.example.test:8080',has_proxy_password:true});h.view.openSettings();edit(h,'Proxy password','draft-secret');h.render({proxy_url:'http://new.example.test:8080'},2);fire(h.find('Discard changes'),'click');
 assert.equal(h.input('Proxy URL').value,'http://new.example.test:8080');assert.equal(h.input('Proxy password').value,'');h.view.dispose();
});

test('Remote HTTP clear-password request is explicit and form errors preserve inputs',async()=>{
 const h=harness({proxy_url:'http://proxy.example.test:8080',has_proxy_password:true});h.view.openSettings();edit(h,'Clear stored password',true);h.ctx.mutate=async work=>{await work();throw Error('Certificate file is invalid');};
 fire(h.form(),'submit');await tick();assert.equal(h.requests[0].body.clear_proxy_password,true);assert.equal(h.requests[0].body.proxy_password,'');assert.equal(h.input('Clear stored password').checked,true);assert.equal(h.all().find(n=>n.getAttribute('role')==='alert').textContent,'Certificate file is invalid');h.view.dispose();
});

test('Remote HTTP pending save blocks repeated requests and runtime CA errors remain visible',async()=>{
 const h=harness({configuration_error:'CA certificate file is unavailable'});h.view.openSettings();assert.ok(h.find('CA certificate file is unavailable'));edit(h,'Proxy URL','https://proxy.example.test:8443');let finish,calls=0;h.ctx.api=()=>{calls++;return new Promise(resolve=>finish=resolve);};
 fire(h.form(),'submit');assert.equal(h.input('Proxy URL').disabled,true);fire(h.form(),'submit');assert.equal(calls,1);finish({});await tick();assert.equal(h.input('Proxy URL').disabled,false);h.view.dispose();
});

test('reloading certificates refreshes saved state without discarding proxy drafts or passwords',async()=>{
 const h=harness();h.view.openSettings();edit(h,'Proxy URL','socks5://proxy.example.test:1080');edit(h,'Proxy password','draft-only');let calls=0;h.ctx.refresh=async()=>{calls++;h.render({configuration_error:'CA file is invalid'},2);};
 fire(h.find('Reload certificates'),'click');await tick();assert.equal(calls,1);assert.equal(h.requests.length,0);assert.equal(h.input('Proxy password').value,'draft-only');assert.equal(h.input('Proxy URL').value,'socks5://proxy.example.test:1080');assert.ok(h.find('CA file is invalid'));h.view.dispose();
});

test('web CA editor is available remotely, blocks drafts and preserves settings during polling',async()=>{
 const h=harness();h.render({},1,{remote_admin:{hostname:'remote-test'}});h.view.openSettings();const browser=h.find('Edit file');assert.equal(browser.disabled,false);
 edit(h,'Proxy URL','http://proxy.example.test:8080');assert.equal(browser.disabled,true);fire(h.find('Discard changes'),'click');
 assert.equal(h.input('Additional CA certificate file').placeholder,'~/.config/zelport/additional-ca.pem');
 const calls=[];h.ctx.ui.fileEditor=(_ctx,options)=>{calls.push(options);return {dispose(){}};};fire(browser,'click');fire(browser,'click');assert.equal(calls.length,1);assert.equal(calls[0].endpoint,'/api/modules/remote_http/ca-file');h.render({},2);assert.equal(browser.disabled,true);calls[0].onClose();assert.equal(browser.disabled,false);h.view.dispose();
});

test('opening Remote HTTP Overview retains a configuration draft for explicit return',()=>{
 const h=harness();h.view.openSettings();const draft=edit(h,'Proxy URL','http://draft.example.test:8080');h.view.openOverview();
 assert.equal(tree(h.toolbar).find(n=>n.textContent==='Overview').getAttribute('aria-current'),'page');h.view.openSettings();assert.equal(h.input('Proxy URL'),draft);assert.equal(draft.value,'http://draft.example.test:8080');h.view.dispose();
});

test('proxy URL errors mentioning a path focus the proxy field, never the CA path',async()=>{
 const h=harness();h.view.openSettings();const proxy=edit(h,'Proxy URL','ftp://proxy.example.test');
 h.ctx.mutate=async()=>{throw Error('Proxy URL must be an HTTP, HTTPS, SOCKS5 or SOCKS5H endpoint without credentials, path, query or fragment');};
 fire(h.form(),'submit');await tick();assert.equal(document.activeElement,proxy);assert.equal(proxy.getAttribute('aria-invalid'),'true');
 edit(h,'Proxy URL','http://proxy.example.test');assert.equal(proxy.getAttribute('aria-invalid'),undefined);h.view.dispose();
});
