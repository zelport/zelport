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
 const context={renderRuntimeHeaderStatus,renderRuntimeAvailability,document:{getElementById:element,body:element('body'),hidden:false,addEventListener(){},querySelectorAll(){return dialogs}},window:{addEventListener(){}},location:{hash:''},history:{pushState(){},replaceState(){}},clearTimeout(){},setTimeout(){},ui:{setTooltip(){},setText,renderChanged,el(){return {append(){}}}}};
 const source=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8').replace(/^import .*;$/gm,'').replace(/init\(\)\.catch\(e=>\{[^\n]*\}\);/,'');
 runInNewContext(source+`\ncomposed=true;let testRefresh=async()=>{};client={refresh:(force)=>testRefresh(force),mutate:async(work,{reportError=true}={})=>{const owner=active;message('',owner);try{const result=await work();await testRefresh(true);return result;}catch(e){if(reportError)message(e.message,owner);await testRefresh(true).catch(()=>{});throw e;}},state:()=>state};globalThis.shell={navigate,message,mutate,refreshVisible,setRefresh(fn){testRefresh=fn},renderAvailability,renderHeaderStatus,addModule(m){modules.set(m.descriptor.id,m)},setState(s){state=s},active(){return active}};`,context);
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
 assert.equal(s.active(),'settings');assert.equal(m.panel.hidden,true);assert.equal(deactivated,1);
});
test('polling can hide the current tab and return it when enabled again',()=>{
 const s=shell();const m={descriptor:{id:'test',tab_id:'test'},panel:{},tab:{setAttribute(){}},instance:{activate(){},deactivate(){}}};
 s.addModule(m);s.setState({modules:[m.descriptor],module_availability:{test:{enabled:true}}});s.navigate('test');
 s.setState({modules:[m.descriptor],module_availability:{test:{enabled:false}}});s.renderAvailability();assert.equal(s.active(),'settings');assert.equal(m.tab.hidden,true);
 s.setState({modules:[m.descriptor],module_availability:{test:{enabled:true}}});s.renderAvailability();s.navigate('test');assert.equal(s.active(),'test');assert.equal(m.panel.hidden,false);
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

test('remote administration uses a distinct page canvas in light and dark themes',()=>{
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8').replace(/\s+/g,'');
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8').replace(/\s+/g,'');
 assert.match(app,/document\.body\.dataset\.administration=state\.remote_admin\?'remote':'local'/);
 assert.match(css,/body\[data-administration=remote\]\{background:#fff4f4\}/);
 assert.match(css,/body\[data-administration=remote\]\{background:#211719\}/);
});

test('Settings keeps the concise module availability label without explanatory filler',()=>{
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 assert.match(app,/ui\.accordion\('Service sources'\)/);
 assert.doesNotMatch(app,/Choose which service sources|Changes apply immediately|source can be hidden/);
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


test('source pages place their toolbar in the shared heading; top-level pages have no heading',()=>{
 const node=()=>({hidden:false,children:[],append(...children){for(const child of children){if(child.parentElement)child.parentElement.children=child.parentElement.children.filter(item=>item!==child);this.children.push(child);child.parentElement=this;}}});
 const s=shell(),moduleToolbar=node(),moduleRefresh={className:'module-action-icon refresh-icon'};moduleToolbar.append(moduleRefresh);
 s.addModule({descriptor:{id:'discovery',tab_id:'discovery'},panel:{},toolbar:moduleToolbar});
 s.navigate('discovery');assert.equal(moduleToolbar.hidden,false);assert.equal(moduleToolbar.parentElement,s.moduleActions);assert.equal(s.moduleHeading.hidden,false);
 for(const page of ['dashboard','sources','settings']){s.navigate(page);assert.equal(s.moduleHeading.hidden,true);assert.equal(moduleToolbar.hidden,true);}
});

test('changing views closes open dialogs, but polling the same view preserves them',()=>{
 const s=shell();let closed=0;s.dialogs.push({close(){closed++}});
 s.navigate('dashboard',false);assert.equal(closed,0);
 s.navigate('settings',false);assert.equal(closed,1);
});


test('header Open settings invokes module configuration after navigation',()=>{
 const calls=[],make=()=>({children:[],dataset:{},style:{},addEventListener(){},setAttribute(){},append(...children){this.children.push(...children)},querySelectorAll(){return []},replaceChildren(){},hidePopover(){calls.push('closed')},matches(){return false}});
 const module={descriptor:{id:'ports_over_relay',name:'Ports over Relay',tab_id:'relay'},headerStatus:()=>({status:'Not configured',tone:'neutral',rows:[]}),instance:{openSettings(){calls.push('configuration')}}},fallback=make(),host=make();host.querySelector=()=>fallback;
 const ui={setTooltip:()=>({update(){},close(){}}),el:make,statusBadge:make,setText,renderChanged,button:(text,click)=>({...make(),click})};
 renderRuntimeHeaderStatus({state:{states:{ports_over_relay:{}}},modules:new Map([['ports_over_relay',module]]),moduleEnabled:()=>true,navigate:id=>calls.push(id),ui,host,positionPopovers(){}});
 module.indicator.settings.click();assert.deepEqual(calls,['closed','relay','configuration']);
});

test('Dashboard keeps route-notice discovery active; source pages activate only their own module',()=>{
 const s=shell(),activated=[],deactivated=[];
 const notice={descriptor:{id:'local',tab_id:'local'},panel:{},instance:{routeNotice(){return ''},activate(){activated.push('local')},deactivate(){deactivated.push('local')}}};
 const plain={descriptor:{id:'remote',tab_id:'remote'},panel:{},instance:{activate(){activated.push('remote')},deactivate(){deactivated.push('remote')}}};
 s.addModule(notice);s.addModule(plain);s.setState({modules:[notice.descriptor,plain.descriptor],module_availability:{}});
 s.navigate('dashboard');assert.deepEqual(activated,['local']);assert.deepEqual(deactivated,['remote']);
 activated.length=deactivated.length=0;s.navigate('remote');assert.deepEqual(activated,['remote']);assert.deepEqual(deactivated,['local']);
 s.navigate('sources');assert.equal(s.active(),'dashboard','the retired Sources page falls back to Dashboard');
});

test('the header shows the Zelport switcher instead of per-module connection indicators',async()=>{
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 assert.match(app,/createSwitcher\(\{host:\$\('connection-status'\),ui\}\)/);
 assert.doesNotMatch(app,/renderRuntimeHeaderStatus\(\{/);
 const {switcherModel,localAdministrationURL}=await import('../internal/web/assets/zelport-switcher.js');
 assert.equal(switcherModel({states:{},runtime:{hostname:'desktop'}}),null,'no relay, no chip');
 assert.equal(switcherModel({states:{ports_over_relay:{data:{enabled:false}}},runtime:{hostname:'desktop'}}),null);
 const peers=[{device_id:'b',hostname:'nas',uuid_url:'http://b.zelport.localhost:8080',available:false},{device_id:'a',hostname:'build-server',hostname_url:'http://build-server.zelport.localhost:8080',uuid_url:'http://a.zelport.localhost:8080',available:true}];
 assert.deepEqual(switcherModel({states:{ports_over_relay:{data:{enabled:true,administration_peers:peers}}},runtime:{hostname:'desktop'}}),{mode:'local',hostname:'desktop',peers:[{hostname:'build-server',url:'http://build-server.zelport.localhost:8080',available:true},{hostname:'nas',url:'http://b.zelport.localhost:8080',available:false}]});
 assert.deepEqual(switcherModel({states:{},remote_admin:{hostname:'build-server',device_id:'a'}}),{mode:'remote',hostname:'build-server',peers:[]});
 assert.equal(localAdministrationURL({protocol:'http:',port:'7070'}),'http://localhost:7070/');
});
