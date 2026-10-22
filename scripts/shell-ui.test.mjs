import {setText,renderChanged} from '../internal/web/assets/ui.js';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import test from 'node:test';
import assert from 'node:assert/strict';
import {renderHeaderStatus as renderRuntimeHeaderStatus,renderAvailability as renderRuntimeAvailability} from '../internal/web/assets/runtime.js';

// Exercise the production shell with only its DOM and network boundaries stubbed.
function shell(){
 const elements=new Map(),dialogs=[];
 const element=id=>{if(!elements.has(id))elements.set(id,{hidden:false,textContent:'',children:[],setAttribute(){},removeAttribute(){},append(...children){for(const child of children){if(child.parentElement)child.parentElement.children=child.parentElement.children.filter(item=>item!==child);this.children.push(child);child.parentElement=this;}},querySelector(){return element('fallback')},querySelectorAll(){return []}});return elements.get(id)};
 const context={renderRuntimeHeaderStatus,renderRuntimeAvailability,document:{getElementById:element,body:element('body'),hidden:false,addEventListener(){},querySelectorAll(){return dialogs}},window:{addEventListener(){}},location:{hash:''},history:{pushState(){},replaceState(){}},clearTimeout(){},setTimeout(){},ui:{setText,renderChanged,el(){return {append(){}}}}};
 const source=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8').replace(/^import .*;$/gm,'').replace(/init\(\)\.catch\(e=>\{[^\n]*\}\);/,'');
 runInNewContext(source+`\nlet testRefresh=async()=>{};client={refresh:(force)=>testRefresh(force),mutate:async(work,{reportError=true}={})=>{const owner=active;message('',owner);try{const result=await work();await testRefresh(true);return result;}catch(e){if(reportError)message(e.message,owner);await testRefresh(true).catch(()=>{});throw e;}},state:()=>state};globalThis.shell={setRefreshControls(toolbar,button){fallbackToolbar=toolbar;fallbackRefresh=button},navigate,message,mutate,refreshVisible,setRefresh(fn){testRefresh=fn},renderAvailability,renderHeaderStatus,addModule(m){modules.set(m.descriptor.id,m)},setState(s){state=s},active(){return active}};`,context);
 return {...context.shell,dialogs,error:element('error'),settings:element('panel-settings'),moduleActions:element('module-actions'),moduleHeading:element('module-heading')};
}

test('errors remain on their originating tab and return when it is selected again',()=>{
 const s=shell();s.navigate('settings');s.message('Port is occupied');
 assert.equal(s.error.textContent,'Port is occupied');
 s.navigate('dashboard');assert.equal(s.error.hidden,true);
 s.navigate('settings');assert.equal(s.error.textContent,'Port is occupied');assert.equal(s.error.hidden,false);
 s.message('');s.navigate('dashboard');s.navigate('settings');assert.equal(s.error.hidden,true);
});

test('a rejected save after navigation belongs to the tab that started it',async()=>{
 const s=shell();s.navigate('settings');let reject;
 const pending=s.mutate(()=>new Promise((_,r)=>{reject=r}));
 s.navigate('dashboard');reject(Error('Port is occupied'));
 await assert.rejects(pending,/Port is occupied/);assert.equal(s.error.hidden,true);
 s.navigate('settings');assert.equal(s.error.textContent,'Port is occupied');
});

test('a delayed refresh failure does not leak onto the next tab',async()=>{
 const s=shell();s.navigate('settings');let reject;s.setRefresh(()=>new Promise((_,r)=>{reject=r}));
 const pending=s.refreshVisible();s.navigate('dashboard');reject(Error('Connection lost'));await pending;
 assert.equal(s.error.hidden,true);s.navigate('settings');assert.equal(s.error.textContent,'Connection lost');
});

test('dialog-owned errors propagate without a duplicate tab banner',async()=>{
 const s=shell();s.navigate('settings');let refreshed=false;s.setRefresh(async()=>{refreshed=true;});
 await assert.rejects(s.mutate(async()=>{throw Error('Invalid token')},{reportError:false}),/Invalid token/);
 assert.equal(s.error.hidden,true);assert.equal(refreshed,true);
});

test('unavailable deep links redirect to Settings and deactivate the module',()=>{
 const s=shell();let deactivated=0;
 const m={descriptor:{id:'test',tab_id:'test'},panel:{},tab:{setAttribute(){}},instance:{deactivate(){deactivated++}}};
 s.addModule(m);s.setState({modules:[m.descriptor],module_availability:{test:{enabled:false}}});s.navigate('test');
 assert.equal(s.active(),'settings');assert.equal(m.tab.hidden,true);assert.equal(m.panel.hidden,true);assert.equal(deactivated,1);
});
test('polling can hide the current tab and return it when enabled again',()=>{
 const s=shell();const m={descriptor:{id:'test',tab_id:'test'},panel:{},tab:{setAttribute(){}},instance:{activate(){},deactivate(){}}};
 s.addModule(m);s.setState({modules:[m.descriptor],module_availability:{test:{enabled:true}}});s.navigate('test');
 s.setState({modules:[m.descriptor],module_availability:{test:{enabled:false}}});s.renderAvailability();assert.equal(s.active(),'settings');assert.equal(m.tab.hidden,true);
 s.setState({modules:[m.descriptor],module_availability:{test:{enabled:true}}});s.renderAvailability();s.navigate('test');assert.equal(s.active(),'test');assert.equal(m.panel.hidden,false);
});

test('hiding a module closes its header popover; re-enabling restores disconnected status',()=>{
 const s=shell();let open=true;
 const indicator={wrap:{},trigger:{dataset:{},setAttribute(){}},popover:{matches(){return open},hidePopover(){open=false}},status:{dataset:{}},settings:{},details:{querySelectorAll(){return []},replaceChildren(){}}};
 s.addModule({descriptor:{id:'client',name:'Client'},headerStatus(){return {status:'Disconnected',tone:'neutral',rows:[]}},indicator});
 s.setState({states:{client:{}},module_availability:{client:{enabled:false}}});s.renderHeaderStatus();
 assert.equal(indicator.wrap.hidden,true);assert.equal(open,false);
 s.setState({states:{client:{}},module_availability:{client:{enabled:true}}});s.renderHeaderStatus();
 assert.equal(indicator.wrap.hidden,false);assert.equal(indicator.status.textContent,'Disconnected');assert.equal(indicator.trigger.dataset.tone,'neutral');
});


test('entering Settings closes accordions before activating sections; staying preserves them',()=>{
 const s=shell(),outer={open:true},advanced={open:true};
 s.settings.querySelectorAll=()=>[outer,advanced];
 let activated=0;
 const m={descriptor:{id:'relay',ui_placement:'settings'},panel:{},accordion:outer,instance:{activate(){activated++},deactivate(){}}};
 s.addModule(m);
 s.navigate('settings');
 assert.equal(outer.open,false);assert.equal(advanced.open,false);assert.equal(m.panel.hidden,true);assert.equal(activated,0);
 outer.open=true;s.navigate('settings');assert.equal(outer.open,true);assert.equal(m.panel.hidden,false);
 s.navigate('dashboard');s.navigate('settings',false);
 assert.equal(outer.open,false);assert.equal(m.panel.hidden,true);
});

test('header popover content is retained until its displayed lines change',()=>{
 const s=shell();let draws=0,rows=[{label:'Relay',value:'localhost:8090'}];
 const indicator={wrap:{},trigger:{dataset:{},setAttribute(){}},popover:{matches(){return false}},status:{dataset:{}},settings:{},details:{querySelectorAll(){return []},replaceChildren(){draws++}}};
 s.addModule({descriptor:{id:'client',name:'Client'},headerStatus(){return {status:'Connected',tone:'ok',rows}},indicator});
 s.setState({states:{client:{}}});s.renderHeaderStatus();s.renderHeaderStatus();assert.equal(draws,1);
 rows=[{label:'Relay',value:'localhost:8091'}];s.renderHeaderStatus();assert.equal(draws,2);
});

test('module status and runtime module popovers share one visual shell',()=>{
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 const runtime=readFileSync(new URL('../internal/web/assets/runtime.js',import.meta.url),'utf8');
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8');
 assert.match(runtime,/ui\.el\('div','header-popover connection-popover'\)/);
 assert.match(app,/ui\.el\('div','header-popover runtime-modules-popover'\)/);
 const shell=css.match(/\.header-popover\{([^}]+)\}/)?.[1]||'';
 for(const rule of ['position:fixed','width:min(340px,calc(100vw - 32px))','padding:var(--panel-padding)','border-radius:var(--radius-panel)','box-shadow:var(--shadow)'])assert.ok(shell.includes(rule),rule);
 assert.doesNotMatch(css,/\.connection-popover\{position:|\.runtime-modules-popover\{position:/);
});

test('successful refresh clears a restart error on every tab',async()=>{
 const s=shell();s.navigate('settings');s.setRefresh(async()=>{throw Error('Failed to fetch')});
 await s.refreshVisible();assert.equal(s.error.hidden,false);
 s.navigate('dashboard');s.setRefresh(async()=>{});await s.refreshVisible();
 s.navigate('settings');assert.equal(s.error.hidden,true);
});
test('recovery preserves a form error even when it matches the refresh error',async()=>{
 const s=shell();s.navigate('settings');s.message('Failed to fetch');
 s.setRefresh(async()=>{throw Error('Failed to fetch')});await s.refreshVisible();
 s.setRefresh(async()=>{});await s.refreshVisible();
 assert.equal(s.error.hidden,false);assert.equal(s.error.textContent,'Failed to fetch');
});


test('fallback Refresh is shown only on the Dashboard',()=>{
 const node=()=>({hidden:false,children:[],append(...children){for(const child of children){if(child.parentElement)child.parentElement.children=child.parentElement.children.filter(item=>item!==child);this.children.push(child);child.parentElement=this;}},prepend(...children){this.append(...children);for(const child of children){this.children=this.children.filter(item=>item!==child);this.children.unshift(child);}},querySelector(selector){return this.children.find(child=>selector==='.refresh-icon'&&child.className?.includes('refresh-icon'))||null;}});
 const s=shell(),toolbar=node(),button={className:'module-action-icon refresh-icon',setAttribute(){}};toolbar.append(button);
 s.setRefreshControls(toolbar,button);
 s.navigate('dashboard');assert.equal(toolbar.hidden,false);assert.equal(button.disabled,false);
 s.navigate('settings');assert.equal(toolbar.hidden,true);assert.equal(button.parentElement,toolbar);
 const moduleToolbar=node(),moduleRefresh={className:'module-action-icon refresh-icon'};moduleToolbar.append(moduleRefresh);
 s.addModule({descriptor:{id:'discovery',tab_id:'discovery'},panel:{},toolbar:moduleToolbar});
 s.navigate('discovery');assert.equal(toolbar.hidden,true);assert.equal(moduleToolbar.hidden,false);assert.equal(moduleToolbar.parentElement,s.moduleActions);assert.equal(s.moduleHeading.hidden,false);
 const staticToolbar=node(),add={className:'module-action-icon module-add-icon'};staticToolbar.append(add);
 s.addModule({descriptor:{id:'static',tab_id:'static'},panel:{},toolbar:staticToolbar});
 s.navigate('static');assert.equal(toolbar.hidden,true);assert.equal(button.disabled,true);assert.equal(staticToolbar.parentElement,s.moduleActions);
 assert.equal(button.parentElement,toolbar);assert.deepEqual(staticToolbar.children,[add]);
 assert.equal(button.title,'Nothing to refresh');
 s.navigate('static');assert.equal(button.parentElement,toolbar);assert.deepEqual(staticToolbar.children,[add]);
 s.navigate('dashboard');assert.equal(button.parentElement,toolbar);assert.deepEqual(toolbar.children,[button]);assert.equal(s.moduleHeading.hidden,true);
});

test('changing views closes open dialogs, but polling the same view preserves them',()=>{
 const s=shell();let closed=0;s.dialogs.push({close(){closed++}});
 s.navigate('dashboard',false);assert.equal(closed,0);
 s.navigate('settings',false);assert.equal(closed,1);
});
test('enabling a module refreshes the header action label without a reload',()=>{
 const s=shell(),indicator={wrap:{},trigger:{dataset:{},setAttribute(){}},popover:{matches(){return false}},status:{dataset:{}},settings:{textContent:'Open settings'},details:{querySelectorAll(){return []},replaceChildren(){}}};
 s.addModule({descriptor:{id:'local_ports'},headerStatus(){return {status:'Available',tone:'ok',action:'Open module',rows:[]}},indicator});
 s.setState({states:{local_ports:{}},module_availability:{local_ports:{enabled:true}}});s.renderHeaderStatus();
 assert.equal(indicator.settings.textContent,'Open module');
});
