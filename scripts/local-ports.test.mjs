import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {listenerPicker} from '../internal/modules/local_ports/assets/module.js';

class Node {
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events=new Map();this.className='';this.textContent='';this.hidden=false;this.disabled=false;this.value='';}
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
 closest(selector){for(let node=this;node;node=node.parentElement)if(selector==='dialog'&&node.tag==='dialog')return node;return null;}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(child=>child!==this);this.removed=true;}
 getBoundingClientRect(){return {top:100,bottom:124,left:100,width:24,height:24};}
 showPopover(){this.open=true;}
 hidePopover(){this.open=false;}
}

function setup(){
 const listeners=new Map(),windowListeners=new Map();
 globalThis.document={
  body:new Node('body'),activeElement:null,
  createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag),
  addEventListener:(name,fn)=>{const handlers=listeners.get(name)||[];handlers.push(fn);listeners.set(name,handlers);},
  removeEventListener:(name,fn)=>listeners.set(name,(listeners.get(name)||[]).filter(handler=>handler!==fn)),
  querySelectorAll:()=>[]
 };
 globalThis.window={innerWidth:1280,innerHeight:800,addEventListener:(name,fn)=>windowListeners.set(name,fn)};
 return {listeners,windowListeners};
}

function walk(node){return [node,...node.children.flatMap(walk)];}
function byClass(node,name){return walk(node).find(item=>item.className.split(' ').includes(name));}
function allByClass(node,name){return walk(node).filter(item=>item.className.split(' ').includes(name));}
function fire(node,type,extra={}){node.dispatchEvent({type,detail:1,preventDefault(){},stopPropagation(){},...extra});}
const flush=()=>new Promise(resolve=>setImmediate(resolve));

function createPicker(api){
 const {windowListeners}=setup();
 const port=ui.field('Aux port',3000,'number'),controller=new AbortController();
 const picker=listenerPicker({moduleId:'local_ports',api,signal:controller.signal,ui},port);
 return {port,picker,controller,windowListeners};
}

test('persistent port stepper exposes vertical increment and decrement controls',()=>{
 setup();const port=ui.numberStepperField('Local (Aux) port',3000,{min:1,max:65535});
 assert.equal(port.input.className,'number-stepper-input');assert.equal(port.input.min,1);assert.equal(port.input.max,65535);
 assert.equal(port.increment.getAttribute('aria-label'),'Increase Local (Aux) port');assert.equal(port.decrease.getAttribute('aria-label'),'Decrease Local (Aux) port');
 assert.equal(walk(port.increment).find(node=>node.tagName==='PATH').getAttribute('d'),'M5 15l7-7 7 7z');assert.equal(walk(port.decrease).find(node=>node.tagName==='PATH').getAttribute('d'),'M5 9l7 7 7-7z');
 let events=0;port.input.addEventListener('input',()=>events++);fire(port.increment,'click');assert.equal(port.input.value,'3001');fire(port.decrease,'click');assert.equal(port.input.value,'3000');assert.equal(events,2);
});

test('listener inventory loads immediately and only its explicit action fills the Aux port',async()=>{
 let calls=0;
 const listeners=[
  {port:8080,processes:[{pid:42,name:'web'}],listens_beyond_loopback:true},
  {port:9000,processes:[],containers:[{name:'zelport-fixture-two-ports',engine:'docker',cli:'docker',runtime:'available',container_port:80,protocol:'tcp'}],listens_beyond_loopback:false},
  {port:9100,processes:[{pid:2345,name:'command -p 2345 --user foo --verbose'}],listens_beyond_loopback:false},
  {port:9200,processes:[{pid:58167,name:'sbx'}],sandboxes:[{name:'project',sandbox_port:3000,protocol:'tcp'}],listens_beyond_loopback:false}
 ];
 const {port,picker,windowListeners}=createPicker(async(path,options)=>{
  calls++;assert.ok(options.signal);
  if(path.endsWith('/refresh')){assert.equal(options.method,'POST');return {attribution_generation:2};}
  assert.equal(path,'/api/modules/local_ports/listeners');assert.equal(options.method,'GET');return {listeners};
 });
 assert.equal(calls,1);assert.equal(port.input.value,3000);assert.equal(port.input.disabled,false);
 assert.equal(port.input.type,'number');assert.equal(byClass(port.wrap,'listener-picker-trigger'),undefined);
 assert.equal(byClass(picker.menu,'listener-picker-header').children[0].textContent,'Detected listening ports');
 await flush();
 assert.equal(calls,1);assert.equal(picker.menu.hidden,false);
 const rows=allByClass(picker.menu,'listener-option');assert.deepEqual(rows.map(row=>row.dataset.port),['8080','9000','9100','9200']);
 const shortProcess=byClass(rows[0],'listener-process');assert.equal(shortProcess.textContent,'web');assert.equal(byClass(shortProcess,'text-tooltip'),undefined);assert.equal(rows[0].getAttribute('aria-label'),'Port 8080: web');
 const localInfo=byClass(rows[0],'local-port-indicator');assert.ok(localInfo,'a direct host listener has a dedicated local-port indicator');
 const localTip=byClass(localInfo,'local-port-tooltip');assert.equal(localTip.textContent,'Kind: Local process\nProcess: web\nPID: 42');
 const warning=byClass(rows[0],'service-warning'),choose=byClass(rows[0],'listener-use-port'),actionOrder=localInfo.parentElement.children;
 assert.deepEqual([warning,localInfo,choose].map(node=>actionOrder.indexOf(node)),[0,1,2],'warning precedes the kind indicator, which is immediately before Choose');
 const containerProcess=byClass(rows[1],'listener-process');assert.equal(containerProcess.textContent,'zelport-fixture-two-…');assert.equal(containerProcess.getAttribute('aria-label'),'zelport-fixture-two-ports');
 const containerInfo=byClass(rows[1],'container-indicator');assert.ok(containerInfo,'a matched container has a dedicated informational indicator');
 const containerTip=byClass(containerInfo,'container-tooltip');assert.equal(containerTip.textContent,'Kind: Container\nContainer: zelport-fixture-two-ports\nMapping: 80/tcp → localhost:9000\nCLI: docker\nRuntime: Docker (available)');
 const longProcess=byClass(rows[2],'listener-process'),processTooltip=byClass(longProcess,'text-tooltip');assert.equal(longProcess.textContent,'command -p 2345 --us…');assert.equal(longProcess.getAttribute('aria-label'),'command -p 2345 --user foo --verbose');
 assert.equal(processTooltip.textContent,'command -p 2345 --user foo --verbose');fire(longProcess,'mouseenter');assert.equal(processTooltip.hidden,true);assert.notEqual(processTooltip.open,true);assert.ok(windowListeners.get('scroll'));windowListeners.get('scroll')();await new Promise(resolve=>setTimeout(resolve,275));assert.equal(processTooltip.hidden,true);fire(longProcess,'mouseenter');await new Promise(resolve=>setTimeout(resolve,275));assert.equal(processTooltip.hidden,false);assert.equal(processTooltip.open,true);fire(longProcess,'mouseleave');assert.equal(processTooltip.hidden,true);
 const sandboxProcess=byClass(rows[3],'listener-process');assert.equal(sandboxProcess.textContent,'project');assert.equal(sandboxProcess.getAttribute('aria-label'),undefined);
 const sandboxInfo=byClass(rows[3],'sandbox-indicator');assert.ok(sandboxInfo,'a matched Docker Sandbox has a dedicated informational indicator');
 const sandboxTip=byClass(sandboxInfo,'sandbox-tooltip');assert.equal(sandboxTip.textContent,'Kind: Docker Sandbox\nSandbox: project\nMapping: 3000/tcp → localhost:9200\nCLI: sbx\nRuntime: Docker Sandbox');
 assert.ok(walk(rows[0]).some(node=>node.textContent==='This port also listens on non-loopback interfaces.'));

 let inputEvents=0;port.input.addEventListener('input',()=>inputEvents++);fire(rows[0],'click');
 assert.equal(port.input.value,3000,'the row itself must not change the field');
 const useButtons=allByClass(picker.menu,'listener-use-port');
 assert.equal(useButtons[0].textContent,'');assert.equal(useButtons[0].getAttribute('aria-label'),'Choose port 8080');assert.equal(useButtons[0].title,'Choose port 8080');
 const chooseIcon=walk(useButtons[0]).find(node=>node.tagName==='SVG');assert.ok(chooseIcon,'the action uses an explicit choose icon instead of clipboard copy');assert.ok(useButtons[0].className.includes('adornment-button'),'Choose uses the same transparent utility control as Refresh');assert.equal(useButtons[0].className.includes('secondary'),false);fire(useButtons[0],'click');
 assert.equal(port.input.value,'8080');assert.equal(inputEvents,1);assert.equal(document.activeElement,port.input);assert.equal(picker.menu.hidden,false);

 const refresh=byClass(picker.menu,'listener-picker-refresh');fire(refresh,'click');await flush();
 assert.equal(calls,3);assert.equal(port.input.value,'8080');picker.dispose();
});

test('listener picker shows spinner then a non-blocking retry state',async()=>{
 let rejectRequest;
 const request=new Promise((_,reject)=>{rejectRequest=reject;});
 const {port,picker}=createPicker(()=>request);
 assert.ok(byClass(picker.menu,'boot-spinner'),'loading state must use the shared spinner');
 rejectRequest(new Error('private operating-system diagnostic'));await flush();
 const message=byClass(picker.menu,'listener-picker-message');
 assert.equal(message.textContent,"Listening ports couldn't be detected. Enter the Aux port manually.");
 assert.equal(port.input.disabled,false);assert.ok(byClass(picker.menu,'listener-picker-refresh'));
 picker.dispose();
});

test('background refresh preserves rows and selection until the completed snapshot replaces them',async()=>{
 let finishRefresh;
 const refreshPending=new Promise(resolve=>{finishRefresh=resolve;});
 let getCount=0,refreshSignal;
 const {port,picker}=createPicker(async(path,options)=>{
  if(path==='/api/modules/local_ports/listeners'){
   getCount++;
   return getCount===1
    ? {attribution_health:'ready',listeners:[{port:1212,kind:'container',source_status:'ready',processes:[{pid:4973,name:'gvproxy'}],containers:[{name:'demo',engine:'podman',cli:'podman',runtime:'available',container_port:80,protocol:'tcp'}],listens_beyond_loopback:true}]}
    : {attribution_health:'ready',listeners:[{port:1313,kind:'container',source_status:'ready',processes:[{pid:4973,name:'gvproxy'}],containers:[{name:'demo',engine:'podman',cli:'podman',runtime:'available',container_port:81,protocol:'tcp'}],listens_beyond_loopback:true}]};
  }
  assert.equal(path,'/api/modules/local_ports/listeners/refresh');assert.equal(options.method,'POST');refreshSignal=options.signal;
  return refreshPending;
 });
 await flush();
 let rows=allByClass(picker.menu,'listener-option');assert.deepEqual(rows.map(row=>row.dataset.port),['1212']);
 fire(allByClass(picker.menu,'listener-use-port')[0],'click');assert.equal(port.input.value,'1212');
 fire(picker.refresh,'click');await flush();
 rows=allByClass(picker.menu,'listener-option');assert.deepEqual(rows.map(row=>row.dataset.port),['1212'],'a blocked refresh must retain the cached rows');
 assert.equal(port.input.value,'1212');assert.equal(allByClass(picker.menu,'listener-use-port')[0].disabled,false);assert.ok(refreshSignal);
 finishRefresh({attribution_health:'ready',attribution_generation:2});await flush();await flush();
 rows=allByClass(picker.menu,'listener-option');assert.deepEqual(rows.map(row=>row.dataset.port),['1313']);assert.equal(port.input.value,'1212','refresh must not overwrite the selected Aux port');
 picker.dispose();
});

test('unknown kind is explained only for unconfirmed attribution while stale positive evidence keeps its kind',async()=>{
 const {picker}=createPicker(async()=>({attribution_health:'stale',listeners:[
  {port:7000,kind:'unknown',source_status:'degraded',processes:[{pid:7,name:'proxy'}],listens_beyond_loopback:false},
  {port:8000,kind:'container',source_status:'stale',processes:[],containers:[{name:'cached',engine:'podman',cli:'podman',runtime:'available',container_port:80,protocol:'tcp',verified_at:'2026-10-21T10:00:00Z'}],listens_beyond_loopback:false}
 ]}));
 await flush();
 const rows=allByClass(picker.menu,'listener-option'),unknown=byClass(rows[0],'unknown-source-indicator');
 assert.ok(unknown);assert.equal(byClass(unknown,'unknown-source-tooltip').textContent,'Kind: Unknown source\nStatus: Discovery is temporarily unavailable.');
 assert.equal(byClass(rows[1],'unknown-source-indicator'),undefined);const container=byClass(rows[1],'container-indicator');assert.ok(container);
 assert.equal(byClass(container,'container-tooltip').textContent,'Kind: Container\nContainer: cached\nMapping: 80/tcp → localhost:8000\nCLI: podman\nRuntime: Podman (available)\nStatus: Stale\nLast verified: 2026-10-21T10:00:00Z');
 picker.dispose();
});

test('disposing during background refresh aborts the waiting refresh request without clearing rows',async()=>{
 let refreshSignal;
 const pending=new Promise(()=>{});
 const {picker}=createPicker(async(path,options)=>{
  if(path.endsWith('/refresh')){refreshSignal=options.signal;return pending;}
  return {listeners:[{port:8080,kind:'local_process',source_status:'ready',processes:[{pid:8,name:'web'}],listens_beyond_loopback:false}]};
 });
 await flush();fire(picker.refresh,'click');await flush();assert.equal(allByClass(picker.menu,'listener-option').length,1);
 picker.dispose();assert.equal(refreshSignal.aborted,true);assert.equal(allByClass(picker.menu,'listener-option').length,1);
});

test('disposing a listener picker aborts discovery and ignores a late response',async()=>{
 let resolveRequest,requestSignal;
 const request=new Promise(resolve=>{resolveRequest=resolve;});
 const {picker}=createPicker((_path,options)=>{requestSignal=options.signal;return request;});
 assert.ok(byClass(picker.menu,'boot-spinner'));
 picker.dispose();assert.equal(requestSignal.aborted,true);
 resolveRequest({listeners:[{port:7777,processes:[],listens_beyond_loopback:false}]});await flush();
 assert.equal(allByClass(picker.menu,'listener-option').length,0);
});
