import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount as relayMount} from '../internal/modules/ports_over_relay/assets/module.js';
import {createDashboard} from '../internal/web/assets/dashboard.js';
import {renderHeaderStatus as renderRuntimeHeaderStatus,renderAvailability as renderRuntimeAvailability} from '../internal/web/assets/runtime.js';
test('initial boot hides the incomplete shell and delays an accessible spinner',()=>{
 const html=readFileSync(new URL('../internal/web/assets/index.html',import.meta.url),'utf8');
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8');
 assert.match(html,/<body data-view="loading">/);
 assert.match(app,/bootStatus\.id='boot-status'/);
 assert.match(app,/bootStatus\.setAttribute\('role','status'\)/);
 assert.match(app,/bootStatus\.setAttribute\('aria-label','Loading Zelport'\)/);
 assert.match(app,/spinner\.className='boot-spinner'/);
 assert.match(css,/body\[data-view=loading\]\s+\.shell\s*\{[^}]*visibility:hidden/);
 assert.match(css,/body\[data-view=loading\]\s+#boot-status\s*\{[^}]*animation:[^;}]*250ms/);
 assert.match(css,/\.boot-spinner[^}]*animation:none/);
});
test('dialog headers vertically center their title and close control',()=>{
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8');
 const header=[...css.matchAll(/\.dialog-header\{([^}]+)\}/g)].at(-1)?.[1]||'';
 assert.match(header,/align-items:center/);
});
class Node {
 constructor(tag){this.tag=tag;this.children=[];this.dataset={};this.style={};this.textContent='';this.className='';this.events={};}
 get lastElementChild(){return this.children.at(-1);}
 prepend(...nodes){this.children.unshift(...nodes);}
 querySelector(selector){return this.children.find(n=>n.className===selector.slice(1));}
 append(...nodes){for(const node of nodes)node.parentElement=this;this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 setAttribute(name,value){this[name]=value;}
 addEventListener(name,fn){this.events[name]=fn;}
 removeEventListener(name){delete this.events[name];}
 querySelectorAll(){return [];}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(n=>n!==this);}
}
globalThis.document={createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),removeEventListener(){}};
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

for(const id of ['local_ports','remote_http','ports_over_ssh']){
 test(`${id}: identical polls and metadata retain nodes; changed routes render`,async()=>{
  const {mount}=await import(`../internal/modules/${id}/assets/module.js`);const ctx=context(),view=mount(ctx);
  const route={id:'one',module:id,domain:'one.localhost',editor:{},summary:{state:'stale',detail:'',target:'localhost'}};
  const data={sandboxes:[],engines:[],updated_at:'first'};view.render({revision:1,routes:[route],data});const before=tree(ctx.panel);
  ctx.setState({port:8080,revision:2});view.render({revision:2,routes:[structuredClone(route)],data:{...data,updated_at:'second'}});
  sameNodes(tree(ctx.panel),before);
  view.render({revision:2,routes:[{...route,domain:'two.localhost'}],data});assert.ok(tree(ctx.panel).some(n=>!before.includes(n)));
 });
}

for(const id of ['local_ports','remote_http','ports_over_ssh'])test(`${id}: unconfigured services use the shared empty-state card`,async()=>{
 const {mount}=await import(`../internal/modules/${id}/assets/module.js`),ctx=context(),view=mount(ctx);
 view.render({routes:[],data:{}});
 const card=tree(ctx.panel).find(n=>n.className.startsWith('empty-state '));
 assert.ok(card,`expected shared empty card for ${id}`);
 const message=tree(card).find(n=>n.className==='empty-state-message');
 assert.ok(message&&message.textContent);
 assert.equal(tree(card).some(n=>n.className==='empty-state-label'||n.tag==='h3'),false);
 const action=tree(card).find(n=>n.tag==='button');
 assert.equal(action.textContent,'+ Add service');
 if(id==='remote_http')assert.equal(message.textContent,'No remote HTTP services yet.');
 const before=tree(ctx.panel);view.render({routes:[],data:{}});sameNodes(tree(ctx.panel),before);
});

test('relay client preserves separate sections and updates expiration without new data',t=>{
 let now=1000;t.mock.method(Date,'now',()=>now);
 const ctx=context(),view=relayMount(ctx),provider={group:'group',device_id:'other',hostname:'peer',port:8080,public_key:'key',publish:true,expires_at:2,services:[{id:'service',kind:'application',domain:'app.localhost'}]};
 const state={revision:1,routes:[],data:{device_id:'self',has_identity:true,enabled:true,publish:false,administration_peers:[],status:{connected:true,records:[provider]}}};
 view.render(state);const services=tree(ctx.panel).find(n=>n.className==='relay-services-view'),setup=tree(ctx.panel).find(n=>n.className==='relay-setup');
 const [,routes,admin,catalog]=services.children,original=tree(ctx.panel),setupBefore=tree(setup);
 view.render({...state,revision:2,data:{...state.data,status:{...state.data.status,metadata_counter:7}}});sameNodes(tree(ctx.panel),original);
 now=3000;view.render(state);
 assert.equal(services.children[1],routes);assert.equal(services.children[2],admin);
 assert.equal(tree(catalog).find(n=>n.tag==='button').disabled,true);
 sameNodes(tree(setup),setupBefore);
});

test('relay client separates service lists from inline configuration',()=>{
 const ctx=context(),view=relayMount(ctx);
 view.render({revision:1,routes:[],data:{has_identity:false,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}});
 const nodes=tree(ctx.panel),services=nodes.find(n=>n.className==='relay-services-view'),configuration=nodes.find(n=>n.className==='relay-configuration-view');
 assert.ok(services&&configuration,'the module needs distinct service and configuration views');
 assert.equal(services.hidden,false);assert.equal(configuration.hidden,true);
 const configure=nodes.find(n=>n.tag==='button'&&n.textContent==='Configuration');
 configure.events.click({preventDefault(){}});
 assert.equal(services.hidden,true);assert.equal(configuration.hidden,false);
 assert.ok(tree(configuration).some(n=>n.tag==='form'),'configuration uses an inline form');
 assert.ok(tree(configuration).some(n=>n.textContent==='E2E secret'));
});

test('relay client hides a revealed E2E secret when leaving Configuration',async()=>{
 const ctx=context();ctx.signal=new AbortController().signal;ctx.api=async()=>({group_token:'test-only'});ctx.mutate=work=>work();
 const view=relayMount(ctx);view.render({revision:1,routes:[],data:{has_identity:true,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}});view.activate();
 const nodes=tree(ctx.panel),configuration=nodes.find(n=>n.tag==='button'&&n.textContent==='Configuration'),services=nodes.find(n=>n.tag==='button'&&n.textContent==='Services');
 configuration.events.click({preventDefault(){}});
 const reveal=tree(ctx.panel).find(n=>n['aria-label']==='Reveal E2E secret');await reveal.events.click({preventDefault(){}});
 assert.ok(tree(ctx.panel).some(n=>n.value==='test-only'));
 services.events.click({preventDefault(){}});
 assert.equal(tree(ctx.panel).some(n=>n.value==='test-only'),false);
});

test('relay configuration can discard a stale draft after a conflict',()=>{
 const ctx=context(),view=relayMount(ctx);view.render({revision:1,routes:[],data:{relay_url:'wss://old.example.test',has_identity:false,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}});
 tree(ctx.panel).find(n=>n.tag==='button'&&n.textContent==='Configuration').events.click({preventDefault(){}});
 const form=tree(ctx.panel).find(n=>n.className==='relay-configuration-form'),url=tree(form).find(n=>n.tag==='input'&&n.value==='wss://old.example.test');
 url.value='wss://draft.example.test';form.events.input({});
 view.render({revision:2,routes:[],data:{relay_url:'wss://new.example.test',has_identity:false,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}});
 assert.equal(url.value,'wss://draft.example.test','polling must preserve the draft');
 tree(ctx.panel).find(n=>n.tag==='button'&&n.textContent==='Discard changes').events.click({preventDefault(){}});
 assert.ok(tree(ctx.panel).some(n=>n.tag==='input'&&n.value==='wss://new.example.test'));
});

test('relay configuration preserves unsaved secret fields when switching views',()=>{
 const ctx=context(),view=relayMount(ctx);view.render({revision:1,routes:[],data:{relay_url:'wss://relay.example.test',has_identity:false,has_access_token:false,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}});
 const configuration=tree(ctx.panel).find(n=>n.tag==='button'&&n.textContent==='Configuration'),services=tree(ctx.panel).find(n=>n.tag==='button'&&n.textContent==='Services');
 configuration.events.click({preventDefault(){}});
 const form=tree(ctx.panel).find(n=>n.className==='relay-configuration-form'),secrets=tree(form).filter(n=>n.tag==='input'&&n.type==='password');
 assert.equal(secrets.length,2);secrets[0].value='draft-access';secrets[1].value='draft-group';form.events.input({});
 services.events.click({preventDefault(){}});configuration.events.click({preventDefault(){}});
 assert.deepEqual(secrets.map(n=>n.value),['draft-access','draft-group']);
 view.deactivate();assert.deepEqual(secrets.map(n=>n.value),['draft-access','draft-group']);
});

test('relay configuration retries a conflicted draft with the latest revision',async()=>{
 const ctx=context(),view=relayMount(ctx),requests=[];
 let state={revision:1,routes:[],data:{relay_url:'wss://old.example.test',has_identity:false,has_access_token:false,enabled:false,publish:false,administration_peers:[],status:{connected:false,records:[]}}};
 ctx.setState(state);view.render(state);tree(ctx.panel).find(n=>n.tag==='button'&&n.textContent==='Configuration').events.click({preventDefault(){}});
 const form=tree(ctx.panel).find(n=>n.className==='relay-configuration-form'),url=tree(form).find(n=>n.tag==='input'&&n.value==='wss://old.example.test');
 url.value='wss://draft.example.test';form.events.input({});
 ctx.api=async(_path,options)=>{const body=JSON.parse(options.body);requests.push(body);if(requests.length===1){const error=Error('Conflict');error.status=409;throw error;}return {};};
 ctx.mutate=async work=>{try{return await work();}catch(error){state={...state,revision:2,data:{...state.data,relay_url:'wss://new.example.test'}};ctx.setState(state);view.render(state);throw error;}};
 await form.events.submit({preventDefault(){}});
 assert.ok(tree(form).some(n=>n.textContent?.includes('Save again')),'the conflict must explain how to retry without losing the draft');
 assert.equal(url.value,'wss://draft.example.test');
 await form.events.submit({preventDefault(){}});
 assert.deepEqual(requests.map(r=>r.revision),[1,2]);assert.equal(requests[1].relay_url,'wss://draft.example.test');
});

test('Dashboard retains rows and runtime nodes across polling and uptime changes',()=>{
 const nodes=new Map(),get=id=>{if(!nodes.has(id))nodes.set(id,new Node('section'));return nodes.get(id)};
 const context={ui,createDashboard,renderRuntimeHeaderStatus,renderRuntimeAvailability,document:{getElementById:get,body:new Node('body'),addEventListener(){},querySelectorAll(){return []}},window:{addEventListener(){},innerHeight:800},history:{replaceState(){}},setTimeout(){},clearTimeout(){},location:{hash:''}};
 const source=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8').replace(/^import .*;$/gm,'').replace(/init\(\)\.catch\(e=>\{[^\n]*\}\);/,'');
 runInNewContext(source+`\nglobalThis.update=s=>{state=s;client={state:()=>state};dashboard??=createDashboard({root:$('dashboard-services'),modules,client,navigate,ui});render()};`,context);
 const state={port:8080,modules:[],states:{test:{routes:[{id:'one',module:'test',domain:'test.localhost',summary:{state:'ready',target:'localhost'}},{id:'remote',module:'remote_http',domain:'remote.localhost',summary:{state:'configured',target:'https://example.test',detail:'Certificate verification is disabled'}}]}},runtime:{version:'dev',hostname:'test-host',built_at:'unknown',uptime_seconds:1,listeners:[],config_path:'test'}};
 context.update(state);const rows=tree(get('dashboard-services')),runtime=tree(get('runtime-info'));
 assert.equal(rows.filter(n=>n.tag==='thead').length,1);assert.equal(rows.filter(n=>n.tag==='tbody').length,1);
 assert.ok(rows.find(n=>n.tag==='table').className.includes('data-table'));
 assert.deepEqual(rows.filter(n=>n.tag==='th').map(n=>n.textContent),['Domain','Source type','Data flow','Actions']);
 assert.ok(!rows.some(n=>n.className.includes('status-badge')||n.className.includes('service-warning')),'Dashboard must not show state badges or warnings');
 assert.ok(rows.some(n=>n.tag==='button'&&n['aria-label']==='Actions for test.localhost'&&n.textContent==='⋮'));
 assert.ok(rows.some(n=>n.tag==='button'&&n.textContent==='Edit'&&n.role==='menuitem'));
 assert.deepEqual(get('runtime-info').children.map(c=>c.dataset.label),['Version','Hostname','Configuration','Listeners','Modules']);
 const moduleCell=get('runtime-info').children.at(-1),moduleNodes=tree(moduleCell);assert.ok(moduleCell.className.includes('runtime-modules'));
 assert.ok(moduleNodes.some(n=>n.className.includes('runtime-modules-trigger')&&n.textContent==='Core only'));
 context.update({...state,revision:2,runtime:{...state.runtime,built_at:'2099-01-01 00:00:00 UTC',uptime_seconds:61}});sameNodes(tree(get('dashboard-services')),rows);sameNodes(tree(get('runtime-info')),runtime);
 context.update({...state,modules:[{id:'stable',name:'Stable module',maturity:'stable'},{id:'preview',name:'Preview module',maturity:'experimental'}]});
 assert.ok(tree(moduleCell).some(n=>n.className.includes('runtime-modules-trigger')&&n.textContent==='Show modules (2)'));
 assert.ok(tree(moduleCell).some(n=>n.textContent==='Stable module'));assert.ok(tree(moduleCell).some(n=>n.className.includes('maturity-experimental')));
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

test('Dashboard shell omits the retired marketing and endpoint introduction',()=>{
 const html=readFileSync(new URL('../internal/web/assets/index.html',import.meta.url),'utf8');
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8');
 for(const token of ['class="intro"','id="endpoint"','MADE FOR LOCALHOST','MAGIC FUNNEL ADDRESS','Unified entry point'])assert.ok(!html.includes(token),token);
 assert.doesNotMatch(css,/\.intro\b|\.endpoint\b|\.hero-mesh\b/);
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
