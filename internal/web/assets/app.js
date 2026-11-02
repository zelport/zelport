import * as ui from './ui.js';
import {createClient} from './client.js';
import {createNavigation} from './navigation.js';
import {createDashboard,sourceLabel} from './dashboard.js';
// Services menu order: Local, SSH, HTTP, Relay (the Add service tiles use the same order).
const sourceOrder=['local_ports','ports_over_ssh','remote_http','ports_over_relay'];
import {createSwitcher,renderRemoteBanner} from './zelport-switcher.js';
import {renderHeaderStatus as renderRuntimeHeaderStatus,renderAvailability as renderRuntimeAvailability} from './runtime.js';
const $=id=>document.getElementById(id);

let state, active='dashboard', timer, client, composed=false;
const modules=new Map(),messages=new Map(),refreshErrors=new Map(),availabilityControls=new Map();
let availabilitySaving=false;
let navigation,dashboard,switcher,runtimeModules,remoteFileAccess,remoteFileAccessSaving=false;
const pages=['dashboard','settings'];
function moduleEnabled(m){return m.descriptor.ui_placement==='settings'||state?.module_availability?.[m.descriptor.id]?.enabled!==false;}
function renderMessage(){const text=messages.get(active)||refreshErrors.get(active)||'';$('error').hidden=!text;ui.setText($('error'),text);}
function message(text,owner=active){if(text)messages.set(owner,text);else messages.delete(owner);renderMessage();}
// Read failures expire on recovery; mutation failures remain owned by their form/tab.
function clearRefreshErrors(){refreshErrors.clear();renderMessage();}
async function refreshVisible(){const owner=active;try{await refresh();clearRefreshErrors();}catch(e){refreshErrors.set(owner,e.message);renderMessage();}}
function api(path,options={}){return client.api(path,options);}
function refresh(force=false){return client.refresh(force);}
function mutate(work,options={}){return client.mutate(work,options);}
function moduleVisible(m){return moduleEnabled(m)&&(m.descriptor.ui_placement==='settings'?active==='settings'&&(!m.accordion||m.accordion.open):m.descriptor.tab_id===active);}
// Dashboard shows module route notices, so their discovery stays active there.
function moduleActive(m){return moduleVisible(m)||active==='dashboard'&&moduleEnabled(m)&&!!m.instance?.routeNotice;}
function sourcePage(id){return [...modules.values()].find(m=>m.descriptor.tab_id===id&&m.descriptor.ui_placement!=='settings');}
function navigate(id,push=true){
 // init reads the latest hash after all module controls are mounted.
 if(!composed)return;
 const page=sourcePage(id),valid=pages.includes(id)||!!page&&moduleEnabled(page);
 const unavailable=!!page&&!moduleEnabled(page);
 const previous=active;active=valid?id:unavailable?'settings':'dashboard';
 if(previous!==active)for(const dialog of document.querySelectorAll('dialog[open]'))dialog.close();
 if(active==='settings'&&previous!=='settings')for(const accordion of $('panel-settings').querySelectorAll('details'))accordion.open=false;
 if(unavailable)history.replaceState(null,'','#settings');
 document.body.setAttribute('data-view',active);
 renderMessage();
 if(push&&location.hash!==`#${active}`)history.pushState(null,'',`#${active}`);
 for(const m of modules.values()){
  const visible=moduleVisible(m);m.panel.hidden=!visible;if(m.toolbar)m.toolbar.hidden=!visible||active==='settings';
  if(m.tab){m.tab.hidden=!moduleEnabled(m);m.tab.setAttribute('aria-current',visible?'page':'false');m.tab.tabIndex=0;}
  if(moduleActive(m))m.instance?.activate();else m.instance?.deactivate();
 }
 const currentModule=sourcePage(active);
 for(const name of pages){
  const visible=active===name;
  $(`panel-${name}`).hidden=!visible;
  $(`tab-${name}`).setAttribute('aria-current',visible?'page':'false');$(`tab-${name}`).tabIndex=0;
 }
 const heading=$('module-heading'),actions=$('module-actions'),title=$('module-title');heading.hidden=!currentModule;
 if(currentModule?.toolbar&&currentModule.toolbar.parentElement!==actions)actions.append(currentModule.toolbar);title.hidden=!currentModule;
 ui.renderChanged(title,[currentModule?.descriptor.name,currentModule?.descriptor.maturity],()=>{title.textContent=currentModule?.descriptor.name||'';title.removeAttribute('aria-label');if(currentModule?.descriptor.maturity==='experimental'){title.append(ui.experimentalBadge());title.setAttribute('aria-label',`${currentModule.descriptor.name} (Experimental)`);}});
 navigation?.sync(active);
 if(active==='dashboard')dashboard?.render(state);
 schedule();
}
function schedule(){
 clearTimeout(timer);if(document.hidden||!state)return;
 const descriptors=state.modules.filter(d=>{const m=modules.get(d.id);return m&&moduleEnabled(m)&&(active==='dashboard'||(d.ui_placement==='settings'?active==='settings':d.tab_id===active));});
 const intervals=descriptors.map(d=>d.poll_interval/1e6).filter(v=>v>0);
 if([...modules.values()].some(m=>m.headerStatus))intervals.push(5000);
 // Service lists follow changes made elsewhere (another tab, an agent over the API) without a click.
 if(active==='dashboard'||sourcePage(active))intervals.push(5000);
 const interval=intervals.length?Math.min(...intervals):0;
 if(interval)timer=setTimeout(async()=>{await refreshVisible();schedule();},interval);
}
// Native popovers live in the top layer; position them against their own trigger.
function positionPopover(trigger,popover){
 const edge=16,gap=8,width=document.documentElement.clientWidth,height=window.innerHeight;
 popover.style.maxHeight=`${Math.max(0,height-edge*2)}px`;
 const anchor=trigger.getBoundingClientRect(),box=popover.getBoundingClientRect();
 const left=Math.max(edge,Math.min(anchor.right-box.width,width-edge-box.width));
 let top=anchor.bottom+gap;
 if(top+box.height>height-edge&&anchor.top-gap-box.height>=edge)top=anchor.top-gap-box.height;
 popover.style.left=`${left}px`;popover.style.top=`${Math.max(edge,Math.min(top,height-edge-box.height))}px`;
}
function positionHeaderPopovers(){
 for(const m of modules.values()){
  const i=m.indicator;if(!i||!i.popover.matches(':popover-open'))continue;
  positionPopover(i.trigger,i.popover);
 }
 if(runtimeModules?.popover.matches?.(':popover-open'))positionPopover(runtimeModules.trigger,runtimeModules.popover);
}
window.addEventListener('resize',positionHeaderPopovers);
window.addEventListener('scroll',positionHeaderPopovers,true);

// The header names where you are (Zelport switcher); connection details live on the source pages.
function renderHeaderStatus(){switcher??=createSwitcher({host:$('connection-status'),ui});switcher.render(state);}
function openAddService(trigger){return dashboard.openAddService(trigger);}
function render(){
 if(!state)return;renderAvailability();renderRemoteFileAccess();renderHeaderStatus();document.body.dataset.administration=state.remote_admin?'remote':'local';
 renderRemoteBanner({banner:$('remote-administration'),state,ui});
 for(const [id,m]of modules){try{m.instance?.render({...state.states[id],revision:state.revision});}catch(e){m.panel.replaceChildren(ui.el('p','route-error',e.message));}}
 dashboard.render(state);for(const m of modules.values())m.services?.render(state);
 for(const m of modules.values()){const count=m.tab?.querySelector?.('.services-menu-count');if(count){ui.setText(count,String(state.states[m.descriptor.id]?.routes?.length||0));if(m.tab.lastElementChild!==count)m.tab.append(count);}}
 ui.setText($('footer-version'),footerVersion(state));
 const rt=state.runtime,fields=[['Version',rt.version],['Hostname',rt.hostname],['Configuration',rt.config_display_path||rt.config_path],['Listeners',rt.listeners?.join(', ')],['Modules',state.modules.map(m=>m.name).join(', ')||'Core only']];
 for(const [label,value]of fields){let cell=[...$('runtime-info').children].find(c=>c.dataset.label===label);if(!cell){cell=ui.el('div',label==='Modules'?'runtime-modules':null);cell.dataset.label=label;cell.append(ui.el('dt',null,label),ui.el('dd'));$('runtime-info').append(cell);}if(label==='Modules')renderRuntimeModules(cell);else ui.setText(cell.children[1],value);if(label==='Configuration'){cell.children[1].tabIndex=0;ui.setTooltip(cell.children[1],rt.config_path||'');}}
}
// The footer names the running build so testers see it on every page.
function footerVersion(s){const version=s.runtime?.version;return version?`Zelport ${version}`:'';}
function renderRuntimeModules(cell){
 if(!runtimeModules){
  const value=cell.lastElementChild,root=ui.el('div','runtime-modules-disclosure'),trigger=ui.el('button','runtime-modules-trigger'),popover=ui.el('div','header-popover runtime-modules-popover');
  trigger.type='button';trigger.setAttribute('aria-haspopup','true');trigger.setAttribute('aria-expanded','false');popover.id='runtime-modules-popover';popover.setAttribute('popover','auto');popover.setAttribute('aria-labelledby','runtime-modules-heading');trigger.setAttribute('popovertarget',popover.id);
  popover.addEventListener('beforetoggle',event=>{trigger.setAttribute('aria-expanded',String(event.newState==='open'));if(event.newState==='open')requestAnimationFrame(positionHeaderPopovers);});root.append(trigger,popover);value.append(root);runtimeModules={trigger,popover};
 }
 const descriptors=state.modules.map(({id,name,maturity})=>({id,name,maturity})),count=descriptors.length,{trigger,popover}=runtimeModules;
 ui.setText(trigger,count?`Show modules (${count})`:'Core only');trigger.disabled=!count;trigger.setAttribute('aria-label',count?`Show ${count} included module${count===1?'':'s'}`:'No modules included');
 ui.renderChanged(popover,descriptors,()=>{
  const heading=ui.el('strong','runtime-modules-heading','Included modules');heading.id='runtime-modules-heading';const list=ui.el('ul','runtime-modules-list');
  for(const descriptor of descriptors){const item=ui.el('li'),name=ui.el('span',null,descriptor.name);item.append(name);if(descriptor.maturity==='experimental')item.append(ui.experimentalBadge());list.append(item);}
  popover.replaceChildren(heading,list);
 });
}
function renderAvailability(){
 const before=active;renderRuntimeAvailability({state,modules,controls:availabilityControls,saving:availabilitySaving,active,navigate,ui});
 if(before!==active&&active==='settings')history.replaceState(null,'','#settings');
}
function addAvailability(settings){
 const section=ui.accordion('Service sources');
 const intro=ui.el('div','field-label-row availability-intro');intro.append(ui.el('span','availability-intro-label','Shown in Services and Add service'),ui.infoHint('Hidden sources leave the Services menu and Add service. Their services keep running.','About hidden sources').root);section.content.append(intro);
 for(const d of state.modules.filter(d=>d.ui_placement!=='settings').sort((a,b)=>sourceOrder.indexOf(a.id)-sourceOrder.indexOf(b.id))){
  const control=ui.switchField(d.name),row=ui.el('div','module-availability-row'),info=ui.el('div','module-availability-info'),count=ui.el('span','module-availability-count');
  // Why a switch is locked stays in its accessible description and in an info hint, not as a sentence.
  const reason=ui.el('p','visually-hidden');reason.id=`availability-reason-${d.id}`;const lock=ui.infoHint('','Why this source stays shown');lock.root.hidden=true;
  control.input.setAttribute('aria-describedby',reason.id);if(d.maturity==='experimental')control.wrap.insertBefore(ui.experimentalBadge(),control.input);control.wrap.insertBefore(count,control.input);control.wrap.insertBefore(lock.root,control.input);info.append(control.wrap,reason);row.append(info);section.content.append(row);
  availabilityControls.set(d.id,{input:control.input,reason,count,lock});
  control.input.addEventListener('change',async()=>{
   if(availabilitySaving)return;
   const enabled=control.input.checked,revision=state.revision;availabilitySaving=true;
   for(const r of availabilityControls.values())r.input.disabled=true;
   try{await mutate(()=>api('/api/module-availability',{method:'PUT',body:JSON.stringify({revision,module:d.id,enabled})}));}
   catch{/* The shell displays this Settings error. */}
   finally{availabilitySaving=false;renderAvailability();if(active==='settings'&&document.activeElement===document.body&&!control.input.disabled)control.input.focus();}
  });
 }
 if(!availabilityControls.size)section.content.append(ui.el('p','muted','No sources are included in this build.'));
 settings.append(section.root);
}
// Remote file access is changed only on this computer; remote sessions see it read-only.
function addRemoteFileAccess(settings){
 const section=ui.accordion('Remote administration'),control=ui.switchField('Allow remote file access'),help=ui.el('p','visually-hidden','Lets devices that administer this Zelport remotely change the SSH configuration and CA file paths and edit those files.'),note=ui.el('p','small muted');
 ui.withInfo(control,'Lets devices that administer this Zelport remotely change the SSH configuration and CA file paths and edit those files.');
 // The risk stays visible while the switch is on.
 const risk=ui.el('p','field-hint field-warning remote-file-access-risk');risk.append(ui.icon('warning',{className:'warning-glyph'}),ui.el('span',null,'Remote devices can run commands on this computer.'));risk.hidden=true;
 help.id='remote-file-access-help';note.id='remote-file-access-note';control.input.setAttribute('aria-describedby','remote-file-access-help remote-file-access-note');
 section.content.append(control.wrap,risk,help,note);settings.append(section.root);remoteFileAccess={input:control.input,note,risk};
 control.input.addEventListener('change',async()=>{
  if(remoteFileAccessSaving)return;remoteFileAccessSaving=true;renderRemoteFileAccess();
  try{await mutate(()=>api('/api/remote-file-access',{method:'PUT',body:JSON.stringify({revision:state.revision,enabled:control.input.checked})}));}
  catch{/* The shell displays this Settings error. */}
  finally{remoteFileAccessSaving=false;renderRemoteFileAccess();}
 });
}
function renderRemoteFileAccess(){
 if(!remoteFileAccess||!state)return;const remote=!!state.remote_admin;
 if(!remoteFileAccessSaving)remoteFileAccess.input.checked=!!state.remote_file_access;
 remoteFileAccess.input.disabled=remote||remoteFileAccessSaving;if(remoteFileAccess.risk)remoteFileAccess.risk.hidden=!remoteFileAccess.input.checked;
 ui.setText(remoteFileAccess.note,remote?'Change this on the computer itself.':'');remoteFileAccess.note.hidden=!remote;
}
function addAbout(settings){const section=ui.accordion('About'),facts=ui.el('dl','runtime-facts');facts.id='runtime-info';section.content.append(facts);settings.append(section.root);}
function tab(id,label){const b=ui.button(label,()=>{navigate(id);refreshVisible();},'');b.id=`tab-${id}`;b.dataset.navigationLabel=label;b.className='navigation-item';b.setAttribute('aria-controls',`panel-${id}`);$('tabs').append(b);return b;}
function showBootStatus(){
 if(!document.body)return null;
 const bootStatus=ui.el('div','boot-status');bootStatus.id='boot-status';bootStatus.setAttribute('role','status');bootStatus.setAttribute('aria-label','Loading Zelport');
 const spinner=ui.el('span');spinner.className='boot-spinner';spinner.setAttribute('aria-hidden','true');bootStatus.append(spinner);document.body.prepend(bootStatus);document.body.setAttribute('aria-busy','true');return bootStatus;
}
// The service list refreshes itself; reading listening ports is the expensive part and runs on demand.
async function rescanPorts(){await refreshVisible();modules.get('local_ports')?.instance?.refreshRouteStatus?.();}
// A source page lists its own services (same table as Dashboard) above its configuration.
function addSourceServices(m){
 const {descriptor,panel,toolbar}=m,services=ui.el('section','source-services'),heading=ui.el('h2','source-section-title','Configuration');
 services.setAttribute('aria-label',`${descriptor.name} services`);panel.prepend(services,heading);
 const tools=ui.el('div','table-tools');
 m.services=createDashboard({root:services,modules,client,navigate,ui,moduleId:descriptor.id,returnTo:descriptor.tab_id,tools,onRescan:rescanPorts});
 const add=ui.button('Add service',()=>m.services.openAddService(add),'source-add-service');
 toolbar.append(tools,add);
}
async function init(){
 const bootStatus=showBootStatus();
 for(const link of document.querySelectorAll('[data-tooltip]'))ui.setTooltip(link,link.dataset.tooltip);
 client=createClient({owner:()=>active,onError:message,onState(next){state=next;clearRefreshErrors();if(composed)render();}});
 state=await client.refresh();tab('dashboard','Dashboard');
 // Sources are named by connection everywhere in the UI: Local, SSH, HTTP, Relay.
 for(const d of state.modules)if(d.ui_placement!=='settings')d.name=sourceLabel(d.id,d.name);
 const addService=$('add-service'),tools=ui.el('div','table-tools');tools.id='dashboard-tools';addService.before(tools);
 dashboard=createDashboard({root:$('dashboard-services'),modules,client,navigate,ui,tools,onRescan:rescanPorts});
 addService.addEventListener('click',()=>openAddService(addService));
 ui.moduleHeading($('module-title'));
 const settings=ui.el('section');settings.id='panel-settings';settings.setAttribute('role','region');settings.setAttribute('aria-labelledby','tab-settings');settings.hidden=true;
 settings.append(ui.el('h1','page-title','Settings'));$('panels').append(settings);
 for(const descriptor of state.modules){
  const inSettings=descriptor.ui_placement==='settings',panel=ui.el('section');
  panel.id=inSettings?`settings-${descriptor.id}`:`panel-${descriptor.tab_id}`;panel.dataset.module=descriptor.id;panel.hidden=true;
  let section=null;
  let tabButton=null;
  if(inSettings){section=ui.accordion(descriptor.name,descriptor.maturity);section.content.append(panel);settings.append(section.root);}else{panel.setAttribute('role','region');panel.setAttribute('aria-labelledby','module-title');$('panels').append(panel);tabButton=tab(descriptor.tab_id,descriptor.name);tabButton.dataset.order=String(sourceOrder.indexOf(descriptor.id)<0?99:sourceOrder.indexOf(descriptor.id));tabButton.append(ui.el('span','services-menu-count'));if(descriptor.maturity==='experimental'){tabButton.setAttribute('aria-label',`${descriptor.name} (Experimental)`);tabButton.append(ui.experimentalBadge());}}
  const toolbar=ui.el('div','module-toolbar');toolbar.hidden=true;$('module-actions').append(toolbar);
  const controller=new AbortController(),m={descriptor,panel,toolbar,tab:tabButton,controller,accordion:section?.root};modules.set(descriptor.id,m);
  section?.root.addEventListener('toggle',()=>{const visible=moduleVisible(m);panel.hidden=!visible;if(visible)m.instance?.activate();else m.instance?.deactivate();});
  const style=ui.el('link');style.rel='stylesheet';style.href=`/assets/modules/${descriptor.id}/style.css`;document.head.append(style);
  try{const code=await import(`/assets/modules/${descriptor.id}/module.js`);m.headerStatus=code.headerStatus;m.instance=code.mount({moduleId:descriptor.id,moduleName:descriptor.name,panel,toolbar,signal:controller.signal,api,refresh,mutate,navigate,statusChanged:()=>{if(composed)render();},ui:{...ui,state:()=>state}});}
  catch(e){panel.append(ui.el('p','route-error',`Module interface unavailable: ${e.message}`));}
  if(!inSettings&&m.instance)addSourceServices(m);
 }
 addAvailability(settings);addRemoteFileAccess(settings);addAbout(settings);
 tab('settings','Settings');composed=true;render();navigate(location.hash.slice(1),false);
 navigation=createNavigation({tabs:$('tabs'),onNavigate:navigate});navigation.sync(active);if(bootStatus){document.body.dataset.view='ready';document.body.setAttribute('aria-busy','false');bootStatus.remove();}
}
window.addEventListener('popstate',()=>navigate(location.hash.slice(1),false));window.addEventListener('hashchange',()=>navigate(location.hash.slice(1),false));document.addEventListener('visibilitychange',()=>{schedule();if(!document.hidden)refreshVisible();});window.addEventListener('focus',()=>{if(state)refreshVisible();});window.addEventListener('pagehide',()=>{clearTimeout(timer);navigation?.dispose();dashboard?.dispose();for(const m of modules.values())m.services?.dispose();client?.dispose();for(const m of modules.values()){m.controller.abort();m.instance?.dispose();}});init().catch(e=>{if(document.body){document.body.dataset.view='error';document.body.setAttribute('aria-busy','false');document.getElementById('boot-status')?.remove();}message(e.message);});
