import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount as relayMount} from '../internal/modules/ports_over_relay/assets/module.js';
class Node {
 constructor(tag){this.tag=tag;this.children=[];this.dataset={};this.style={};this.textContent='';this.className='';this.events={};}
 get lastElementChild(){return this.children.at(-1);}
 prepend(...nodes){this.children.unshift(...nodes);}
 querySelector(selector){return this.children.find(n=>n.className===selector.slice(1));}
 append(...nodes){for(const node of nodes)node.parentElement=this;this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 setAttribute(name,value){this[name]=value;}
 addEventListener(name,fn){this.events[name]=fn;}
 querySelectorAll(){return [];}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(n=>n!==this);}
}
globalThis.document={createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag)};
function sameNodes(actual,expected){assert.equal(actual.length,expected.length);actual.forEach((node,i)=>assert.equal(node,expected[i]));}
function tree(n){return [n,...n.children.flatMap(tree)];}
function context(){const panel=new Node('section');let state={port:8080,revision:1};return {panel,moduleId:'test',ui:{...ui,state:()=>state},setState(s){state=s;}};}

test('unchanged text is not assigned again',()=>{
 let writes=0,text='selected';const node={get textContent(){return text},set textContent(value){writes++;text=value}};
 ui.setText(node,'selected');assert.equal(writes,0);ui.setText(node,'changed');assert.equal(writes,1);
});

test('failed renders can be retried with the same content',()=>{
 const node=new Node('section');let calls=0;
 assert.throws(()=>ui.renderChanged(node,{value:1},()=>{throw Error('failed')}));
 ui.renderChanged(node,{value:1},()=>calls++);ui.renderChanged(node,{value:1},()=>calls++);assert.equal(calls,1);
});

for(const id of ['docker_sandboxes','containers','local_ports','remote_http','ports_over_ssh']){
 test(`${id}: identical polls and metadata retain nodes; changed routes render`,async()=>{
  const {mount}=await import(`../internal/modules/${id}/assets/module.js`);const ctx=context(),view=mount(ctx);
  const route={id:'one',module:id,domain:'one.localhost',editor:{},summary:{state:'stale',detail:'',target:'localhost'}};
  const data={sandboxes:[],engines:[],updated_at:'first'};view.render({revision:1,routes:[route],data});const before=tree(ctx.panel);
  ctx.setState({port:8080,revision:2});view.render({revision:2,routes:[structuredClone(route)],data:{...data,updated_at:'second'}});
  sameNodes(tree(ctx.panel),before);
  view.render({revision:2,routes:[{...route,domain:'two.localhost'}],data});assert.ok(tree(ctx.panel).some(n=>!before.includes(n)));
 });
}

test('relay client preserves separate sections and updates expiration without new data',t=>{
 let now=1000;t.mock.method(Date,'now',()=>now);
 const ctx=context(),view=relayMount(ctx),provider={group:'group',device_id:'other',hostname:'peer',port:8080,public_key:'key',publish:true,expires_at:2,services:[{id:'service',kind:'application',domain:'app.localhost'}]};
 const state={revision:1,routes:[],data:{device_id:'self',has_identity:true,enabled:true,publish:false,administration_peers:[],status:{connected:true,records:[provider]}}};
 view.render(state);const [setup,,routes,admin,catalog]=ctx.panel.children;const original=tree(ctx.panel);
 view.render({...state,revision:2,data:{...state.data,status:{...state.data.status,metadata_counter:7}}});sameNodes(tree(ctx.panel),original);
 now=3000;view.render(state);
 assert.equal(ctx.panel.children[0],setup);assert.equal(ctx.panel.children[2],routes);assert.equal(ctx.panel.children[3],admin);
 assert.equal(tree(catalog).find(n=>n.tag==='button').disabled,true);
 sameNodes(tree(setup),original.slice(1,1+tree(setup).length));
});

test('Dashboard retains rows and runtime nodes across polling and uptime changes',()=>{
 const nodes=new Map(),get=id=>{if(!nodes.has(id))nodes.set(id,new Node('section'));return nodes.get(id)};
 const context={ui,document:{getElementById:get,body:new Node('body'),addEventListener(){}},window:{addEventListener(){}},setTimeout(){},clearTimeout(){},location:{hash:''}};
 const source=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8').replace("import * as ui from './ui.js';",'').replace('init().catch(e=>message(e.message));','');
 runInNewContext(source+'\nglobalThis.update=s=>{state=s;render()};',context);
 const state={port:8080,modules:[],states:{test:{routes:[{id:'one',module:'test',domain:'test.localhost',summary:{state:'ready',target:'localhost'}}]}},runtime:{version:'dev',built_at:'unknown',uptime_seconds:1,listeners:[],config_path:'test'}};
 context.update(state);const rows=tree(get('dashboard-services')),runtime=tree(get('runtime-info'));
 assert.equal(rows.filter(n=>n.tag==='thead').length,1);assert.equal(rows.filter(n=>n.tag==='tbody').length,1);
 assert.ok(rows.find(n=>n.tag==='table').className.includes('data-table'));
 context.update({...state,revision:2,runtime:{...state.runtime,uptime_seconds:2}});sameNodes(tree(get('dashboard-services')),rows);sameNodes(tree(get('runtime-info')),runtime);
 context.update({...state,runtime:{...state.runtime,uptime_seconds:61}});sameNodes(tree(get('dashboard-services')),rows);sameNodes(tree(get('runtime-info')),runtime);
 assert.equal(get('runtime-info').children.find(c=>c.dataset.label==='Uptime').lastElementChild.textContent,'1 min');
 const stableDomain=tree(get('dashboard-services')).find(n=>n.tag==='a');
 const changed={...state,states:{test:{routes:[{...state.states.test.routes[0],summary:{state:'unavailable',target:'localhost'}}]}}};context.update(changed);
 assert.equal(tree(get('dashboard-services')).find(n=>n.tag==='a'),stableDomain,'a status update must retain the selected domain node');
 for(const status of ['ready','stale','unavailable','waiting','error','configured']){
  const route={...state.states.test.routes[0],summary:{state:status,target:'localhost',detail:'Diagnostic must stay in module UI'}};
  const next={...state,states:{test:{routes:[route]}}};context.update(next);
  assert.ok(!tree(get('dashboard-services')).some(n=>n.textContent===route.summary.detail),status);
  const retained=tree(get('dashboard-services'));context.update({...next,states:{test:{routes:[{...route,summary:{...route.summary,detail:'Updated diagnostic'}}]}}});sameNodes(tree(get('dashboard-services')),retained);
 }

});

for(const lifecycle of ['deactivate','dispose'])test(`relay client discards a late secret response after ${lifecycle}`,async()=>{
 const ctx=context();ctx.signal=new AbortController().signal;let finish;
 ctx.api=()=>new Promise(resolve=>finish=resolve);ctx.mutate=work=>work();
 const view=relayMount(ctx);
 view.render({routes:[],data:{has_identity:true,enabled:false,status:{records:[]}}});view.activate();
 const reveal=tree(ctx.panel).find(n=>n['aria-label']==='Reveal E2E secret');
 const request=reveal.events.click({preventDefault(){}});view[lifecycle]();finish({group_token:'test-only'});await request;await Promise.resolve();
 assert.ok(!tree(ctx.panel).some(n=>n.value==='test-only'));
});

test('container engine details and service list survive unchanged polls independently',async()=>{
 const {mount}=await import('../internal/modules/containers/assets/module.js');const ctx=context(),view=mount(ctx);
 const engine={engine:'podman',connection:'one',cli_status:'found',cli_path:'/test/podman',runtime_status:'available',updated_at:'first',containers:[{id:'container',name:'web',status:'running',ports:[]}]};
 const state={routes:[],data:{engines:[engine]}};view.render(state);
 const [list]=ctx.panel.children;
 const before=tree(ctx.panel),listBefore=tree(list);
 view.render({...state,revision:2,data:{engines:[{...engine,updated_at:'later'}]}});sameNodes(tree(ctx.panel),before);
 view.render({...state,data:{engines:[{...engine,error:'inspection failed',stale:true}]}});
 sameNodes(tree(list),listBefore);
 view.render({...state,data:{engines:[{...engine,containers:[{...engine.containers[0],status:'exited'}]}]}});
 assert.ok(tree(list).some(n=>!listBefore.includes(n)));
});

test('container diagnostics move to header sections without adding inventory rows',async()=>{
 const {mount,headerStatus}=await import('../internal/modules/containers/assets/module.js');const ctx=context(),view=mount(ctx);
 const state={routes:[],data:{engines:[{engine:'docker',cli_status:'not_found',error:'executable not found'},{engine:'podman',cli_status:'found',runtime_status:'available'},{engine:'nerdctl',cli_status:'found',runtime_status:'unavailable',error:'permission denied'}]}};
 view.render(state);assert.equal(ctx.panel.children.length,1);
 const info=headerStatus(state);assert.equal(info.sections.length,3);
 assert.ok(!info.sections[0].rows.some(r=>r.label==='Error'));
 assert.ok(info.sections[2].rows.some(r=>r.value==='permission denied'));
});

test('sandbox diagnostics move to the header while inventory nodes survive polls',async()=>{
 const {mount,headerStatus}=await import('../internal/modules/docker_sandboxes/assets/module.js');
 const ctx=context(),view=mount(ctx),data={cli:{status:'found',version:'v0.48.0',path:'/test/sbx'},stale:false,sandboxes:[]};
 view.render({revision:1,routes:[],data});
 const list=ctx.panel.children[0],original=tree(list);assert.equal(ctx.panel.children.length,1);
 view.render({revision:2,routes:[],data:{...data,updated_at:'new timestamp'}});sameNodes(tree(list),original);
 const failed={revision:3,routes:[],data:{...data,stale:true,error:'Discovery unavailable'}};
 view.render(failed);assert.equal(list.hidden,true);
 assert.equal(headerStatus(failed).status,'Discovery failed');
 assert.ok(headerStatus(failed).rows.some(r=>r.value==='Discovery unavailable'));
 view.render({revision:4,routes:[],data});assert.equal(list.hidden,false);
});
