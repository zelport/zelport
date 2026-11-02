import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import * as ui from '../internal/web/assets/ui.js';
import {createDashboard} from '../internal/web/assets/dashboard.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.attrs={};this.events={};this.disabled=false;this.hidden=false;this.textContent='';this.value='';}
 append(...nodes){for(const node of nodes){node.parentElement=this;this.children.push(node);}}
 setAttribute(name,value){this.attrs[name]=String(value);}
 getAttribute(name){return this.attrs[name];}
 removeAttribute(name){delete this.attrs[name];}
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
 globalThis.document={body:new Node('body'),activeElement:null,createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),addEventListener(){},removeEventListener(){}};
}




const find=(node,cls)=>node.className?.split?.(' ').includes(cls)?node:node.children.map(child=>find(child,cls)).find(Boolean);

test('Dashboard keeps Status for the service state and shows target security warnings beside Target',()=>{
 setup();
 const route=(id,module,detail='')=>({id,module,domain:`${id}.localhost`,summary:{state:'configured',target:`https://${id}.example.test`,detail}});
 const state={port:7070,module_availability:{},modules:[{}],states:{remote_http:{routes:[route('secure','remote_http'),route('unverified','remote_http','TLS certificate verification is disabled.')]}}};
 const modules=new Map([['remote_http',{descriptor:{id:'remote_http',name:'Remote HTTP'},instance:{
  routeStatus:()=>({tone:'neutral',label:'Configured'}),routeWarning:r=>r.summary.detail||''}}]]);
 const root=new Node(),dashboard=createDashboard({root,modules,client:{state:()=>state},navigate(){},ui});
 dashboard.render(state);
 const rows=find(root,'dashboard-table').children[1].children;
 assert.equal(rows.length,2);
 for(const [row,warning] of [[rows[0],''],[rows[1],'TLS certificate verification is disabled.']]){
  const [, , target,status]=row.children;
  assert.equal(find(status,'service-status-label').textContent,'Configured');
  assert.equal(find(status,'service-status').dataset.tone,'neutral');
  assert.equal(find(target,'service-target-text').textContent,row===rows[0]?'https://secure.example.test':'https://unverified.example.test');
  assert.equal(find(target,'service-warning').hidden,!warning);
  assert.equal(find(target,'warning-tooltip').textContent,warning);
  assert.equal(find(target,'service-target-detail').textContent,warning);assert.equal(find(target,'service-target-detail').hidden,!warning);
 }
 // The warning clears when the property goes away, without rebuilding the row.
 state.states.remote_http.routes[1].summary.detail='';dashboard.render(state);
 assert.equal(find(rows[1].children[2],'service-warning').hidden,true);
});

test('Sources report security properties as warnings, not as a status label',()=>{
 const remote=readFileSync(new URL('../internal/modules/remote_http/assets/module.js',import.meta.url),'utf8');
 const local=readFileSync(new URL('../internal/modules/local_ports/assets/module.js',import.meta.url),'utf8');
 assert.doesNotMatch(remote,/label:'Unencrypted'/);assert.match(remote,/routeWarning\(route\)\{return route\.summary\.detail/);
 assert.doesNotMatch(local,/label:'Exposed'/);assert.match(local,/routeWarning\(route\)/);
});

test('A source page lists only its own services, without the Source column, and Add service returns to it',()=>{
 setup();
 const route=(id,module)=>({id,module,domain:`${id}.localhost`,summary:{state:'configured',target:`https://${id}.example.test`}});
 const state={port:7070,module_availability:{},modules:[{}],states:{remote_http:{routes:[route('a','remote_http')]},local_ports:{routes:[route('b','local_ports'),route('c','local_ports')]}}};
 const calls=[],navigated=[],modules=new Map();
 for(const [id,name] of [['local_ports','Local Ports'],['remote_http','Remote HTTP']])modules.set(id,{descriptor:{id,name},instance:{edit:(r,g,options)=>{calls.push(options);return {};}}});
 const root=new Node(),page=createDashboard({root,modules,client:{state:()=>state},navigate:id=>navigated.push(id),ui,moduleId:'local_ports',returnTo:'local'});
 page.render(state);
 const table=find(root,'dashboard-table');
 assert.ok(table.className.includes('source-table'));
 assert.deepEqual(table.children[0].children[0].children.map(cell=>cell.children[0]?.textContent??cell.textContent),['Address','Target','Status','Actions']);
 assert.deepEqual(table.children[1].children.map(row=>row.children[0].children[0].textContent),['b.localhost:7070','c.localhost:7070']);
 page.openAddService({focus(){}});
 assert.equal(calls[0].sourceId,'local_ports');calls[0].onSaved();assert.deepEqual(navigated,['local']);
 state.states.local_ports.routes=[];page.render(state);
 assert.equal(find(root,'dashboard-empty').children[0].textContent,'No services');
});

test('Search matches the service name, or the whole address once the query has a dot or colon',async()=>{
 const {addressMatches}=await import('../internal/web/assets/dashboard.js');
 assert.equal(addressMatches('',"logs.localhost",'logs.localhost:7070'),true);
 assert.equal(addressMatches('LOG','logs.localhost','logs.localhost:7070'),true);
 assert.equal(addressMatches('localhost','logs.localhost','logs.localhost:7070'),false);
 assert.equal(addressMatches('7070','logs.localhost','logs.localhost:7070'),false);
 assert.equal(addressMatches('logs.localh','logs.localhost','logs.localhost:7070'),true);
 assert.equal(addressMatches('.localhost:7070','logs.localhost','logs.localhost:7070'),true);
 assert.equal(addressMatches(':8080','logs.localhost','logs.localhost:7070'),false);
});

test('Status sorting puts problems first and Source uses the short connection names',()=>{
 setup();globalThis.localStorage={store:{},getItem(k){return this.store[k]??null},setItem(k,v){this.store[k]=v}};
 const route=(id,module)=>({id,module,domain:`${id}.localhost`,summary:{state:'configured',target:id}});
 const tones={a:'ok',b:'error',c:'warning',d:'neutral'};
 const state={port:7070,module_availability:{},modules:[{}],states:{local_ports:{routes:['a','b','c','d'].map(id=>route(id,'local_ports'))}}};
 const modules=new Map([['local_ports',{descriptor:{id:'local_ports',name:'Local Ports'},instance:{routeStatus:r=>({tone:tones[r.id],label:r.id})}}]]);
 const root=new Node(),dashboard=createDashboard({root,modules,client:{state:()=>state},navigate(){},ui});
 dashboard.render(state);
 const table=find(root,'dashboard-table'),body=table.children[1],head=table.children[0].children[0];
 assert.equal(find(body.children[0],'service-source').textContent,'Local');
 const statusButton=head.children[3].children[0];assert.equal(statusButton.textContent,'Status');
 body.children=[];statusButton.events.click();
 assert.deepEqual(body.children.filter(row=>row.className!=='no-matches-row').map(row=>row.children[0].children[0].textContent),['b.localhost:7070','c.localhost:7070','d.localhost:7070','a.localhost:7070']);
 assert.equal(head.children[3].attrs['aria-sort'],'ascending');
 assert.deepEqual(JSON.parse(localStorage.store['zelport.sort.dashboard']),{column:'Status',direction:1});
});
