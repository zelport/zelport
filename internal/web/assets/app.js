import * as ui from './ui.js';
import {createClient} from './client.js';
import {createNavigation} from './navigation.js';
import {createDashboard} from './dashboard.js';
import {renderHeaderStatus as renderRuntimeHeaderStatus,renderAvailability as renderRuntimeAvailability} from './runtime.js';
const $=id=>document.getElementById(id);

let state, active='dashboard', timer, client, composed=false;
const modules=new Map(),messages=new Map(),refreshErrors=new Map(),availabilityControls=new Map();
let availabilitySaving=false, fallbackToolbar, fallbackRefresh, fallbackRefreshing=false;
let navigation,dashboard,runtimeModules;
function renderRefreshControl(){
 if(!fallbackToolbar)return;
 if(fallbackRefresh.parentElement!==fallbackToolbar)fallbackToolbar.append(fallbackRefresh);
 fallbackToolbar.hidden=active!=='dashboard';
 const enabled=active==='dashboard';
 fallbackRefresh.disabled=!enabled||fallbackRefreshing;
 const label=enabled?'Refresh services':active==='settings'?'Refresh unavailable while editing settings':'Nothing to refresh';
 fallbackToolbar.title=label;fallbackRefresh.title=label;fallbackRefresh.setAttribute('aria-label',label);
}
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
function navigate(id,push=true){
 const valid=id==='dashboard'||id==='settings'||[...modules.values()].some(m=>moduleEnabled(m)&&m.descriptor.ui_placement!=='settings'&&m.descriptor.tab_id===id);
 const unavailable=[...modules.values()].some(m=>m.descriptor.tab_id===id&&!moduleEnabled(m));
 const previous=active;active=valid?id:unavailable?'settings':'dashboard';
 if(previous!==active)for(const dialog of document.querySelectorAll('dialog[open]'))dialog.close();
 if(active==='settings'&&previous!=='settings')for(const accordion of $('panel-settings').querySelectorAll('details'))accordion.open=false;
 if(unavailable)history.replaceState(null,'','#settings');
 document.body.setAttribute('data-view',active);
 renderMessage();
 if(push&&location.hash!==`#${active}`)history.pushState(null,'',`#${active}`);
 for(const m of modules.values()){
  const visible=moduleVisible(m);m.panel.hidden=!visible;if(m.toolbar)m.toolbar.hidden=!visible||active==='settings';
  if(m.tab)m.tab.hidden=!moduleEnabled(m);
  if(m.tab){m.tab.setAttribute('aria-current',visible?'page':'false');m.tab.tabIndex=0;}
  if(visible)m.instance?.activate();else m.instance?.deactivate();
 }
 for(const name of ['dashboard','settings']){
  const visible=active===name;$( `panel-${name}`).hidden=!visible;
  $(`tab-${name}`).setAttribute('aria-current',visible?'page':'false');$(`tab-${name}`).tabIndex=0;
 }
 const currentModule=[...modules.values()].find(m=>m.descriptor.tab_id===active&&m.descriptor.ui_placement!=='settings');
 const heading=$('module-heading'),actions=$('module-actions'),title=$('module-title');heading.hidden=!currentModule;if(currentModule?.toolbar&&currentModule.toolbar.parentElement!==actions)actions.append(currentModule.toolbar);title.hidden=!currentModule;ui.renderChanged(title,[currentModule?.descriptor.name,currentModule?.descriptor.maturity],()=>{title.textContent=currentModule?.descriptor.name||'';title.removeAttribute('aria-label');if(currentModule?.descriptor.maturity==='experimental'){title.append(ui.experimentalBadge());title.setAttribute('aria-label',`${currentModule.descriptor.name} (Experimental)`);}});
 renderRefreshControl();
 navigation?.sync(active);
 schedule();
}
function schedule(){
 clearTimeout(timer);if(document.hidden||!state)return;
 const descriptors=state.modules.filter(d=>{const m=modules.get(d.id);return m&&moduleEnabled(m)&&(active==='dashboard'||(d.ui_placement==='settings'?active==='settings':d.tab_id===active));});
 const intervals=descriptors.map(d=>d.poll_interval/1e6).filter(v=>v>0);
 if([...modules.values()].some(m=>m.headerStatus))intervals.push(5000);
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

function renderHeaderStatus(){renderRuntimeHeaderStatus({state,modules,moduleEnabled,navigate,ui,host:$('connection-status'),positionPopovers:positionHeaderPopovers});}
function openAddService(trigger){return dashboard.openAddService(trigger);}
function render(){if(!state)return;renderAvailability();renderHeaderStatus();const remote=$('remote-administration');remote.hidden=!state.remote_admin;if(state.remote_admin)ui.setText(remote,`Remote administration · ${state.remote_admin.hostname} · ${state.remote_admin.device_id}. Actions run on this remote device.`);for(const [id,m]of modules){try{m.instance?.render({...state.states[id],revision:state.revision});}catch(e){m.panel.replaceChildren(ui.el('p','route-error',e.message));}}
 dashboard.render(state);
 const rt=state.runtime,fields=[['Version',rt.version],['Hostname',rt.hostname],['Configuration',rt.config_display_path||rt.config_path],['Listeners',rt.listeners?.join(', ')],['Modules',state.modules.map(m=>m.name).join(', ')||'Core only']];
 for(const [label,value]of fields){let cell=[...$('runtime-info').children].find(c=>c.dataset.label===label);if(!cell){cell=ui.el('div',label==='Modules'?'runtime-modules':null);cell.dataset.label=label;cell.append(ui.el('dt',null,label),ui.el('dd'));$('runtime-info').append(cell);}if(label==='Modules')renderRuntimeModules(cell);else ui.setText(cell.lastElementChild,value);if(label==='Configuration')cell.lastElementChild.title=rt.config_path||'';}}
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
 const section=ui.accordion('Module UI availability');
 for(const d of state.modules.filter(d=>d.ui_placement!=='settings')){
  const control=ui.switchField(d.name),row=ui.el('div','module-availability-row'),info=ui.el('div','module-availability-info');
  const reason=ui.el('p','small muted');reason.id=`availability-reason-${d.id}`;
  control.input.setAttribute('aria-describedby',reason.id);if(d.maturity==='experimental')control.wrap.insertBefore(ui.experimentalBadge(),control.input);info.append(control.wrap,reason);row.append(info);section.content.append(row);
  availabilityControls.set(d.id,{input:control.input,reason});
  control.input.addEventListener('change',async()=>{
   if(availabilitySaving)return;
   const enabled=control.input.checked,revision=state.revision;availabilitySaving=true;
   for(const r of availabilityControls.values())r.input.disabled=true;
   try{await mutate(()=>api('/api/module-availability',{method:'PUT',body:JSON.stringify({revision,module:d.id,enabled})}));}
   catch{/* The shell displays this Settings error. */}
   finally{availabilitySaving=false;renderAvailability();if(active==='settings'&&document.activeElement===document.body&&!control.input.disabled)control.input.focus();}
  });
 }
 if(!availabilityControls.size)section.content.append(ui.el('p','muted','No module interfaces are included in this build.'));
 settings.append(section.root);
}
function tab(id,label){const b=ui.button(label,()=>{navigate(id);refreshVisible();},'');b.id=`tab-${id}`;b.dataset.navigationLabel=label;b.className='navigation-item';b.setAttribute('aria-controls',`panel-${id}`);$('tabs').append(b);return b;}
function showBootStatus(){
 if(!document.body)return null;
 const bootStatus=ui.el('div','boot-status');bootStatus.id='boot-status';bootStatus.setAttribute('role','status');bootStatus.setAttribute('aria-label','Loading Zelport');
 const spinner=ui.el('span');spinner.className='boot-spinner';spinner.setAttribute('aria-hidden','true');bootStatus.append(spinner);document.body.prepend(bootStatus);document.body.setAttribute('aria-busy','true');return bootStatus;
}
async function init(){
 const bootStatus=showBootStatus();
 client=createClient({owner:()=>active,onError:message,onState(next){state=next;clearRefreshErrors();if(composed)render();}});
 state=await client.refresh();tab('dashboard','Dashboard');
 dashboard=createDashboard({root:$('dashboard-services'),modules,client,navigate,ui});
 const addService=$('add-service');addService.addEventListener('click',()=>openAddService(addService));
 ui.moduleHeading($('module-title'));
 fallbackToolbar=ui.el('div','module-toolbar');
 fallbackRefresh=ui.refreshControl('Refresh services',async()=>{if(fallbackRefreshing)return;fallbackRefreshing=true;fallbackRefresh.disabled=true;fallbackRefresh.setAttribute('aria-busy','true');try{await refreshVisible();}finally{fallbackRefreshing=false;fallbackRefresh.setAttribute('aria-busy','false');renderRefreshControl();}});
 fallbackToolbar.append(fallbackRefresh);$('tabs').parentElement.append(fallbackToolbar);
 const settings=ui.el('section');settings.id='panel-settings';settings.setAttribute('role','region');settings.setAttribute('aria-labelledby','tab-settings');settings.hidden=true;settings.tabIndex=0;$('panels').append(settings);
 for(const descriptor of state.modules){
  const inSettings=descriptor.ui_placement==='settings',panel=ui.el('section');
  panel.id=inSettings?`settings-${descriptor.id}`:`panel-${descriptor.tab_id}`;panel.dataset.module=descriptor.id;panel.hidden=true;
  let tabButton=null,section=null;
  if(inSettings){section=ui.accordion(descriptor.name,descriptor.maturity);section.content.append(panel);settings.append(section.root);}else{panel.setAttribute('role','region');panel.setAttribute('aria-labelledby',`tab-${descriptor.tab_id}`);$('panels').append(panel);tabButton=tab(descriptor.tab_id,descriptor.name);if(descriptor.maturity==='experimental'){tabButton.setAttribute('aria-label',`${descriptor.name} (Experimental)`);tabButton.append(ui.experimentalBadge());}}
  const toolbar=ui.el('div','module-toolbar');toolbar.hidden=true;$('module-actions').append(toolbar);
  const controller=new AbortController(),m={descriptor,panel,toolbar,tab:tabButton,controller,accordion:section?.root};modules.set(descriptor.id,m);
  section?.root.addEventListener('toggle',()=>{const visible=moduleVisible(m);panel.hidden=!visible;if(visible)m.instance?.activate();else m.instance?.deactivate();});
  const style=ui.el('link');style.rel='stylesheet';style.href=`/assets/modules/${descriptor.id}/style.css`;document.head.append(style);
  try{const code=await import(`/assets/modules/${descriptor.id}/module.js`);m.headerStatus=code.headerStatus;m.instance=code.mount({moduleId:descriptor.id,panel,toolbar,signal:controller.signal,api,refresh,mutate,navigate,ui:{...ui,state:()=>state}});}
  catch(e){panel.append(ui.el('p','route-error',`Module interface unavailable: ${e.message}`));}
 }
 addAvailability(settings);
 tab('settings','Settings');composed=true;render();navigate(location.hash.slice(1),false);
 navigation=createNavigation({tabs:$('tabs'),onNavigate:navigate});navigation.sync(active);if(bootStatus){document.body.dataset.view='ready';document.body.setAttribute('aria-busy','false');bootStatus.remove();}
}
window.addEventListener('popstate',()=>navigate(location.hash.slice(1),false));window.addEventListener('hashchange',()=>navigate(location.hash.slice(1),false));document.addEventListener('visibilitychange',()=>{schedule();if(!document.hidden)refreshVisible();});window.addEventListener('focus',()=>{if(state)refreshVisible();});window.addEventListener('pagehide',()=>{clearTimeout(timer);navigation?.dispose();dashboard?.dispose();client?.dispose();for(const m of modules.values()){m.controller.abort();m.instance?.dispose();}});init().catch(e=>{if(document.body){document.body.dataset.view='error';document.body.setAttribute('aria-busy','false');document.getElementById('boot-status')?.remove();}message(e.message);});
