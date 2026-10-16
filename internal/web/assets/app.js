import * as ui from './ui.js';
const $=id=>document.getElementById(id);
function formatBuildTime(value){
 const match=/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) UTC$/.exec(value||'');
 if(!match)return value||'unknown';
 const date=new Date(`${match[1]}T${match[2]}Z`);
 if(Number.isNaN(date.getTime()))return value;
 const zone=new Intl.DateTimeFormat('en-GB',{timeZoneName:'short'}).formatToParts(date).find(part=>part.type==='timeZoneName').value;
 const pad=value=>String(value).padStart(2,'0');
 return `${pad(date.getDate())}-${pad(date.getMonth()+1)}-${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())} ${zone}`;
}

let state, active='dashboard', pending, timer, busy=false;
const modules=new Map(),messages=new Map(),refreshErrors=new Map(),availabilityControls=new Map();
let availabilitySaving=false, fallbackToolbar, fallbackRefresh, fallbackRefreshing=false;
let navigation;
function renderRefreshControl(){
 if(!fallbackToolbar)return;
 const moduleRefresh=active!=='settings'&&[...modules.values()].some(m=>moduleVisible(m)&&m.toolbar?.querySelector('.refresh-icon'));
 fallbackToolbar.hidden=moduleRefresh;
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
async function api(path,options={}){
 if(!path.startsWith('/api/')||path.includes('://'))throw Error('Invalid API path');
 const method=options.method||'GET';const headers={...options.headers};
 if(!['GET','HEAD'].includes(method)){headers['X-CSRF-Token']=state?.csrf;headers['Content-Type']='application/json';}
 const response=await fetch(path,{...options,headers});const text=await response.text();let data;
 try{data=JSON.parse(text);}catch{data={error:text};}
 if(!response.ok){const error=Error(data.error||`Request failed (${response.status})`);error.status=response.status;throw error;}return data;
}
async function refresh(force=false){if(pending){if(!force)return pending;await pending.catch(()=>{});}pending=(async()=>{state=await api('/api/state');clearRefreshErrors();render();})().finally(()=>pending=null);return pending;}
async function mutate(work,{reportError=true}={}){const owner=active;if(busy)throw Error('Another operation is in progress');busy=true;message('',owner);document.body.setAttribute('aria-busy','true');try{const result=await work();await refresh(true);return result;}catch(e){if(reportError)message(e.message,owner);await refresh(true).catch(()=>{});throw e;}finally{busy=false;document.body.removeAttribute('aria-busy');}}
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
 const title=$('module-title');title.hidden=!currentModule;ui.renderChanged(title,[currentModule?.descriptor.name,currentModule?.descriptor.maturity],()=>{title.textContent=currentModule?.descriptor.name||'';title.removeAttribute('aria-label');if(currentModule?.descriptor.maturity==='experimental'){title.append(ui.experimentalBadge());title.setAttribute('aria-label',`${currentModule.descriptor.name} (Experimental)`);}});
 renderRefreshControl();
 navigation?.select();
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
function positionHeaderPopovers(){
 for(const m of modules.values()){
  const i=m.indicator;if(!i||!i.popover.matches(':popover-open'))continue;
  const edge=16,gap=8,width=document.documentElement.clientWidth,height=window.innerHeight;
  i.popover.style.maxHeight=`${Math.max(0,height-edge*2)}px`;
  const anchor=i.trigger.getBoundingClientRect(),box=i.popover.getBoundingClientRect();
  const left=Math.max(edge,Math.min(anchor.right-box.width,width-edge-box.width));
  let top=anchor.bottom+gap;
  if(top+box.height>height-edge&&anchor.top-gap-box.height>=edge)top=anchor.top-gap-box.height;
  top=Math.max(edge,Math.min(top,height-edge-box.height));
  i.popover.style.left=`${left}px`;i.popover.style.top=`${top}px`;
 }
}
window.addEventListener('resize',positionHeaderPopovers);
window.addEventListener('scroll',positionHeaderPopovers,true);

// Keep popover nodes stable while polling so focus and open state survive updates.
function renderHeaderStatus(){
 const host=$('connection-status');let visible=0;
 for(const m of modules.values()){
  if(!m.headerStatus)continue;
  const value=moduleEnabled(m)?m.headerStatus(state.states[m.descriptor.id]):null;
  if(!m.indicator){
   const wrap=ui.el('div','connection-indicator'),trigger=ui.el('button','connection-trigger',m.descriptor.name),popover=ui.el('div','connection-popover');
   wrap.style.order=String(m.descriptor.order||0);
   popover.id=`connection-${m.descriptor.id}`;popover.setAttribute('popover','auto');
   popover.addEventListener('beforetoggle',event=>{if(event.newState==='open')requestAnimationFrame(positionHeaderPopovers);});
   trigger.type='button';trigger.setAttribute('popovertarget',popover.id);
   const heading=ui.el('strong',null,m.descriptor.name),status=ui.statusBadge(''),details=ui.el('div','connection-details');
   const settings=ui.button(value?.action||'Open settings',()=>{popover.hidePopover();navigate(m.descriptor.ui_placement==='settings'?'settings':m.descriptor.tab_id);if(m.accordion)m.accordion.open=true;const target=m.accordion?.querySelector('summary')||m.tab;target?.focus();},'secondary');
   const top=ui.el('div','connection-popover-heading');top.append(heading,status);const actions=ui.el('div','connection-popover-actions');actions.append(settings);popover.append(top,details,actions);wrap.append(trigger,popover);host.append(wrap);m.indicator={wrap,trigger,popover,status,details,settings};
  }
  const i=m.indicator;i.wrap.hidden=!value;
  if(!value){if(i.popover.matches(':popover-open'))i.popover.hidePopover();continue;}
  visible++;ui.setText(i.settings,value.action||'Open settings');i.trigger.dataset.tone=value.tone;i.trigger.title=`${m.descriptor.name}: ${value.status}`;i.trigger.setAttribute('aria-label',i.trigger.title);
  ui.setText(i.status,value.status);i.status.dataset.tone=value.tone;ui.renderChanged(i.details,value.sections||value.rows,()=>i.details.replaceChildren(...(value.sections||[{rows:value.rows}]).map(section=>{
   const group=ui.el('section','connection-section'),facts=ui.el('dl','connection-facts');
   if(section.title)group.append(ui.el('h3',null,section.title));
   for(const {label,value} of section.rows){const row=ui.el('div');row.append(ui.el('dt',null,label),ui.el('dd',null,value));facts.append(row);}
   group.append(facts);return group;
  })));
 }
 let fallback=host.querySelector('.local-label');if(!fallback){fallback=ui.el('span','local-label');host.prepend(fallback);}
 fallback.hidden=visible>0&&!state.remote_admin;ui.setText(fallback,state.remote_admin?'Remote · E2E':'Local');
 positionHeaderPopovers();
}
// Keep rows and cells attached when other services change during polling.
const dashboardRows=new Map();
let dashboardTable,dashboardBody,dashboardEmpty;
function renderDashboardServices(){
 const routes=Object.values(state.states).flatMap(s=>s.routes||[]),host=$('dashboard-services');
 if(!dashboardTable){
  const wrap=ui.el('div','dashboard-table-wrap');dashboardTable=ui.el('table','dashboard-table data-table');
  const head=ui.el('tr'),thead=ui.el('thead');dashboardBody=ui.el('tbody');
  for(const name of ['Domain','Source type','Data flow','State','Actions']){const cell=ui.el('th',null,name);cell.scope='col';head.append(cell);}
  thead.append(head);dashboardTable.append(thead,dashboardBody);wrap.append(dashboardTable);dashboardEmpty=ui.el('div','empty');host.append(wrap,dashboardEmpty);
 }
 dashboardTable.parentElement.hidden=!routes.length;dashboardEmpty.hidden=!!routes.length;
 if(!routes.length)ui.setText(dashboardEmpty,state.modules.length?'No services configured yet.':'This build contains only the core. Add feature build tags to configure services.');
 const present=new Set(routes.map(r=>r.id));
 for(const [id,item]of dashboardRows)if(!present.has(id)){item.row.remove();dashboardRows.delete(id);}
 for(const r of routes){
  let item=dashboardRows.get(r.id);
  if(!item){
   const row=ui.el('tr'),domain=ui.el('td'),source=ui.el('td'),target=ui.el('td'),status=ui.el('td'),action=ui.el('td');
   for(const [cell,label]of [[domain,'Domain'],[source,'Source type'],[target,'Data flow'],[status,'State'],[action,'Actions']])cell.dataset.label=label;
   const badge=ui.statusBadge(''),warning=ui.warningIndicator(),targetText=ui.el('span');status.append(badge);target.append(targetText,warning.root);item={row,domain,source,target,targetText,status,badge,warning,route:r};dashboardRows.set(r.id,item);
   item.edit=ui.button('Edit',()=>modules.get(item.route.module)?.instance?.edit(item.route),'secondary');action.append(item.edit);
   row.append(domain,source,target,status,action);dashboardBody.append(row);
  }
  item.route=r;item.edit.disabled=r.summary.state==='unsupported';item.edit.title=item.edit.disabled?'Module is not included in this build':'';
  const address=ui.serviceURL(state,r),linked=!!address;
  if(!item.link||item.linked!==linked){
   item.link=ui.el(linked?'a':'span');item.linked=linked;item.domain.replaceChildren(item.link);
   if(linked){item.link.target='_blank';item.link.rel='noopener';}
   else if(r.summary.state!=='unsupported')item.domain.append(ui.el('span','small muted','Not connected on this device'));
  }
  item.link.title=r.summary.state==='unsupported'?'Module is not included in this build. Configuration is preserved.':'';
  ui.setText(item.link,r.domain);if(linked&&item.address!==address)item.link.href=address;item.address=address;
  ui.setText(item.source,modules.get(r.module)?.descriptor.name||r.module);
  ui.setText(item.targetText,r.summary.target||'');ui.setText(item.badge,r.summary.state);item.badge.dataset.tone=ui.statusTone(r.summary.state);const warning=r.module==='remote_http'?r.summary.detail||'':'';item.warning.update(warning);
 }
 // Preserve existing row order across snapshots; newly discovered services append.
}
function render(){if(!state)return;const missing=state.unsupported_modules||[];const notice=$('unsupported-modules');notice.hidden=!missing.length;ui.setText(notice,missing.length?`Modules not included in this build: ${missing.join(', ')}. Their configuration is preserved; services are inactive.`:'');renderAvailability();ui.setText($('gateway'),state.remote_admin?state.remote_admin.authority:`localhost:${state.port}`);renderHeaderStatus();const remote=$('remote-administration');remote.hidden=!state.remote_admin;if(state.remote_admin)ui.setText(remote,`Remote administration · ${state.remote_admin.hostname} · ${state.remote_admin.device_id}. Actions run on this remote device.`);for(const [id,m]of modules){try{m.instance?.render({...state.states[id],revision:state.revision});}catch(e){m.panel.replaceChildren(ui.el('p','route-error',e.message));}}
 renderDashboardServices();
 const rt=state.runtime;for(const [label,value]of Object.entries({Version:rt.version,'Build time':formatBuildTime(rt.built_at),Uptime:`${Math.floor(rt.uptime_seconds/60)} min`,Listeners:rt.listeners?.join(', '),Modules:state.modules.map(m=>m.name).join(', ')||'Core only',Configuration:rt.config_display_path||rt.config_path})){let cell=[...$('runtime-info').children].find(c=>c.dataset.label===label);if(!cell){cell=ui.el('div');cell.dataset.label=label;cell.append(ui.el('dt',null,label),ui.el('dd'));$('runtime-info').append(cell);}ui.setText(cell.lastElementChild,value);if(label==='Configuration')cell.lastElementChild.title=rt.config_path||'';}}
function renderAvailability(){
 for(const [id,row]of availabilityControls){
  const a=state.module_availability?.[id];if(!a)continue;
  row.input.checked=a.enabled;row.input.disabled=availabilitySaving||(a.enabled&&!a.can_disable);
  ui.setText(row.reason,a.reason||`${a.services} configured service${a.services===1?'':'s'}`);
 }
 for(const m of modules.values())if(m.tab)m.tab.hidden=!moduleEnabled(m);
 const current=[...modules.values()].find(m=>m.descriptor.tab_id===active);
 if(current&&!moduleEnabled(current)){navigate('settings',false);history.replaceState(null,'','#settings');}
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
async function init(){
 state=await api('/api/state');tab('dashboard','Dashboard');
 fallbackToolbar=ui.el('div','module-toolbar');
 fallbackRefresh=ui.refreshControl('Refresh services',async()=>{if(fallbackRefreshing)return;fallbackRefreshing=true;fallbackRefresh.disabled=true;fallbackRefresh.setAttribute('aria-busy','true');try{await refreshVisible();}finally{fallbackRefreshing=false;fallbackRefresh.setAttribute('aria-busy','false');renderRefreshControl();}});
 fallbackToolbar.append(fallbackRefresh);$('tabs').parentElement.append(fallbackToolbar);
 const settings=ui.el('section');settings.id='panel-settings';settings.setAttribute('role','region');settings.setAttribute('aria-labelledby','tab-settings');settings.hidden=true;settings.tabIndex=0;$('panels').append(settings);
 for(const descriptor of state.modules){
  const inSettings=descriptor.ui_placement==='settings',panel=ui.el('section');
  panel.id=inSettings?`settings-${descriptor.id}`:`panel-${descriptor.tab_id}`;panel.dataset.module=descriptor.id;panel.hidden=true;
  let tabButton=null,section=null;
  if(inSettings){section=ui.accordion(descriptor.name,descriptor.maturity);section.content.append(panel);settings.append(section.root);}else{panel.setAttribute('role','region');panel.setAttribute('aria-labelledby',`tab-${descriptor.tab_id}`);$('panels').append(panel);tabButton=tab(descriptor.tab_id,descriptor.name);if(descriptor.maturity==='experimental'){tabButton.setAttribute('aria-label',`${descriptor.name} (Experimental)`);tabButton.append(ui.experimentalBadge());}}
  const toolbar=ui.el('div','module-toolbar');toolbar.hidden=true;$('tabs').parentElement.append(toolbar);
  const controller=new AbortController(),m={descriptor,panel,toolbar,tab:tabButton,controller,accordion:section?.root};modules.set(descriptor.id,m);
  section?.root.addEventListener('toggle',()=>{const visible=moduleVisible(m);panel.hidden=!visible;if(visible)m.instance?.activate();else m.instance?.deactivate();});
  const style=ui.el('link');style.rel='stylesheet';style.href=`/assets/modules/${descriptor.id}/style.css`;document.head.append(style);
  try{const code=await import(`/assets/modules/${descriptor.id}/module.js`);m.headerStatus=code.headerStatus;m.instance=code.mount({moduleId:descriptor.id,panel,toolbar,signal:controller.signal,api,refresh,mutate,navigate,ui:{...ui,state:()=>state}});}
  catch(e){panel.append(ui.el('p','route-error',`Module interface unavailable: ${e.message}`));}
 }
 addAvailability(settings);
 tab('settings','Settings');render();navigate(location.hash.slice(1),false);
 navigation=ui.serviceNavigation($('tabs'));
}
window.addEventListener('popstate',()=>navigate(location.hash.slice(1),false));window.addEventListener('hashchange',()=>navigate(location.hash.slice(1),false));document.addEventListener('visibilitychange',()=>{schedule();if(!document.hidden)refreshVisible();});window.addEventListener('focus',()=>{if(state)refreshVisible();});window.addEventListener('pagehide',()=>{clearTimeout(timer);navigation?.dispose();for(const m of modules.values()){m.controller.abort();m.instance?.dispose();}});init().catch(e=>message(e.message));
