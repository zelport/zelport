import {mountConfiguration} from './configuration.js';
function connectionStatus(data) {
 if(!data.relay_url?.trim())return {status:'Not configured',tone:'neutral'};
 if(!data.enabled)return {status:'Disabled',tone:'neutral'};
 const status=data.status||{};
 return status.connected?{status:'Connected',tone:'ok'}:status.error?{status:'Connection failed',tone:'error'}:{status:'Connecting',tone:'pending'};
}
export function connectionMessage(data){
 if(!data.enabled||!data.relay_url?.trim()||data.status?.connected||!data.status?.error)return '';
 const error=data.status.error;
 if(/authentication|frame header|EOF|handshake/i.test(error))return 'Relay handshake failed. Check the access mode and token.';
 if(/relay disconnected/i.test(error))return 'Relay disconnected. Reconnecting…';
 if(/connection failed|dial|refused|timeout|no such host/i.test(error))return 'Cannot reach the relay. Check the URL and server.';
 return 'Relay connection failed. Check the connection settings.';
}
export function headerStatus(state) {
 const d=state?.data;if(!d||!d.relay_url?.trim())return null;
 let endpoint='Not configured';
 try{endpoint=new URL(d.relay_url).host;}catch{}
 const rows=[{label:'Relay',value:endpoint},{label:'Devices',value:String((d.administration_peers||[]).filter(p=>p.available).length)},{label:'Sharing',value:d.publish?'Enabled':'Disabled'},{label:'Imports',value:String(state.routes?.length||0)}];
 const connection=connectionStatus(d);
 if(connection.tone==='error')rows.push({label:'Connection',value:connectionMessage(d)});
 return {...connection,rows};
}
export function mount(ctx) {
 const {el,button,field,serviceModule,renderChanged,statusBadge,refreshButton}=ctx.ui;
 let current,configuration,servicePicker=null,active=false,refreshing=false;
 const overviewView=el('section','relay-services-view'),configurationView=el('section','relay-configuration-view');
 const routePanel=el('section'),setup=el('section','relay-setup'),catalog=el('section','relay-catalog'),settingsPanel=el('section','relay-settings-panel');
 // Imports are listed on Services; routePanel only hosts the shared import form state.
 routePanel.hidden=true;overviewView.append(routePanel,catalog);
 configurationView.append(setup,settingsPanel);configurationView.hidden=true;
 const views=ctx.ui.moduleViews({...ctx,moduleName:ctx.moduleName||'Ports over Relay'},{overview:overviewView,configuration:configurationView,overviewPlacement:'after',onChange(view){
  ensureConfiguration();if(active)configuration?.activate();
 }});
 const api=path=>`/api/modules/${ctx.moduleId}/${path}`;
 const routes=serviceModule({...ctx,panel:routePanel},{
  add:false,
  routeMenu:true,
  columns:['Service','Provider','Info','Actions'],
  tableLabel:'Imported relay services',
  serviceLabel:(route,state)=>`${route.domain}:${state.port}`,
  targetLabel:route=>providerName(route),
  rowState:state=>[(state.data?.status?.records||[]).map(r=>[r.device_id,r.hostname]),(state.data?.administration_peers||[]).map(p=>[p.device_id,p.hostname])],
  createTitle:'Import service',
  createSubmit:'Import service',
  targetInfo:route=>`Provider device: ${route.editor?.device_id||route.summary.target}\nService ID: ${route.editor?.service_id||''}`,
  rowIndicators:route=>route.summary.detail?[ctx.ui.warningIndicator(route.summary.detail,'Service unavailable').root]:[],
  emptyState:{kind:'no-results',message:'No services imported yet.'},fields(settings,group,state,{address}={}){
  if(!settings.device_id)return sharedServicePicker(address);
  const {choiceList,choiceRow,optionSwitch,serviceOptions}=ctx.ui,element=el('div'),list=choiceList('Shared service'),options=serviceOptions(),origin=optionSwitch('Use target origin',settings.rewrite_origin,originHelp);
  const record=current?.data?.status?.records?.find(r=>r.device_id===settings.device_id),service=record?.services?.find(s=>s.id===settings.service_id);
  const choice=choiceRow({primary:service?.domain||settings.service_id||'',secondary:providerName({editor:settings,summary:{target:settings.device_id}}),onSelect(){},disabled:true});choice.select(true);list.append(choice.row);
  options.append(origin.row);element.append(fieldLabel('Shared service'),list,options);
  return {element,read:()=>({...settings,rewrite_origin:origin.input.checked})};
 }});
 // Add service lists the services other devices share; picking one fills the address.
 const originHelp='For services that reject requests from localhost.';
 function fieldLabel(text){return el('p','field-label',text);}
 const pickers=new Set();
 function availableDomain(service){
  const used=new Set(Object.values(ctx.ui.state().states||{}).flatMap(s=>(s.routes||[]).map(r=>r.domain)));
  const base=service.domain.replace(/\.localhost$/,'');let domain=service.domain,n=2;
  while(used.has(domain))domain=`${base}-${n++}.localhost`;return domain;
 }
 function sharedServicePicker(address){
  const {choiceGroup,choiceList,choiceRow,choiceState,optionSwitch,serviceOptions}=ctx.ui;
  const element=el('div','service-fields-fill'),list=choiceList('Shared services'),options=serviceOptions(),origin=optionSwitch('Use target origin',false,originHelp);
  options.append(origin.row);element.append(fieldLabel('Shared service'),list,options);
  let selected=null,drawn='',suggested=address?.value||'';const rows=[];
  const tab=ctx.ui.state().modules?.find(m=>m.id===ctx.moduleId)?.tab_id;
  const settingsLink=()=>{if(!tab)return undefined;const link=el('a','choice-state-link','Relay settings');link.href=`#${tab}`;return link;};
  function pick(entry){selected=entry;for(const row of rows)row.select(row.key===entry.key);
   if(address&&address.value===suggested){suggested=availableDomain(entry.service).replace(/\.localhost$/,'');address.value=suggested;address.dispatchEvent(new Event('input',{bubbles:true}));}}
  function draw(){
   const d=current?.data||{},status=d.status||{},connected=!!status.connected,providers=(status.records||[]).filter(r=>r.device_id!==d.device_id&&r.publish);
   const peers=(d.administration_peers||[]).filter(p=>p.device_id!==d.device_id&&!providers.some(r=>r.device_id===p.device_id));
   const configured=!!d.relay_url?.trim()&&d.enabled,key=JSON.stringify([configured,connected,providers.map(r=>[r.device_id,r.hostname,r.port,r.expires_at*1000>Date.now(),r.services.filter(s=>s.kind==='application').map(s=>[s.id,s.domain])]),peers.map(p=>[p.device_id,p.hostname,p.available])]);
   if(key===drawn)return;drawn=key;const focused=list.contains(document.activeElement)?document.activeElement.dataset?.key:null;
   list.replaceChildren();rows.length=0;
   const badge=online=>{const b=statusBadge(online?'Online':'Offline');b.dataset.tone=online?'ok':'neutral';return b;};
   const names=[...providers,...peers].map(device=>device.hostname),deviceName=device=>names.filter(name=>name===device.hostname).length>1?`${device.hostname} · ${device.device_id.slice(0,8)}`:device.hostname;
   for(const provider of [...providers].sort((a,b)=>a.hostname.localeCompare(b.hostname))){
    const online=connected&&provider.expires_at*1000>Date.now(),services=provider.services.filter(s=>s.kind==='application');
    list.append(choiceGroup(deviceName(provider),badge(online)));
    if(!services.length)list.append(choiceState('No services shared.'));
    for(const service of services){
     const key=`${provider.device_id}/${service.id}`,entry={key,provider,service};
     const choice=choiceRow({primary:provider.port?`${service.domain}:${provider.port}`:service.domain,label:`Import ${service.domain} from ${deviceName(provider)}`,onSelect:()=>pick(entry),disabled:!online});
     choice.row.className+=' relay-shared-option';choice.main.dataset.key=key;choice.select(selected?.key===key);list.append(choice.row);rows.push({...choice,key});
    }
   }
   for(const peer of peers){list.append(choiceGroup(deviceName(peer),badge(peer.available)),choiceState('No services shared.'));}
   const listed=providers.length+peers.length>0;
   if(!listed)list.append(choiceState(!configured?'The relay is not set up.':!connected?'Not connected to the relay.':'No other devices yet.',{action:settingsLink()}));
   else if(!connected)list.prepend(choiceState('Not connected to the relay.',{action:settingsLink()}));
   options.hidden=!listed;
   if(focused)[...list.querySelectorAll('button')].find(button=>button.dataset?.key===focused)?.focus();
  }
  draw();const picker={draw};pickers.add(picker);
  return {element,read(){if(!selected)throw Error('Choose a service to import.');const {provider,service}=selected;return {group:provider.group,device_id:provider.device_id,service_id:service.id,provider_key:provider.public_key,rewrite_origin:origin.input.checked};},dispose(){pickers.delete(picker);}};
 }
 function importService(provider,service){
  const used=new Set(Object.values(ctx.ui.state().states||{}).flatMap(s=>(s.routes||[]).map(r=>r.domain)));
  for(const route of current.routes||[])used.add(route.domain);
  const base=service.domain.replace(/\.localhost$/,'');let domain=service.domain,n=2;
  while(used.has(domain))domain=`${base}-${n++}.localhost`;
  routes.edit({id:'',domain,editor:{group:provider.group,device_id:provider.device_id,service_id:service.id,provider_key:provider.public_key,rewrite_origin:false}});
 }
 function chooseService(deviceId,trigger){
  const provider=current.data?.status?.records?.find(r=>r.device_id===deviceId);
  if(!current.data?.status?.connected||!provider?.publish||provider.expires_at*1000<=Date.now())return;
  const services=provider.services.filter(s=>s.kind==='application');
  if(services.length===1){importService(provider,services[0]);return;}
  if(!services.length||servicePicker)return;
  const dialog=el('dialog','relay-service-picker'),choices=el('div','relay-service-choices');servicePicker=dialog;
  dialog.append(ctx.ui.dialogHeader('Import service',dialog),el('p','small muted',provider.hostname),choices);
  for(const service of services)choices.append(button(`${service.domain}:${provider.port}`,()=>{dialog.close();importService(provider,service);}));
  ctx.ui.showDialog(dialog,{signal:ctx.signal,initialFocus:choices.children[0],onClose(){servicePicker=null;if(trigger.isConnected)trigger.focus();}});
 }
 function providerName(route){return current?.data?.status?.records?.find(r=>r.device_id===route.editor?.device_id)?.hostname||current?.data?.administration_peers?.find(p=>p.device_id===route.editor?.device_id)?.hostname||route.summary.target;}
 function ensureConfiguration(){if(configuration||!current)return;configuration=mountConfiguration(ctx);settingsPanel.append(configuration.element);configuration.render(current);}
 function openSettings(){views.show('configuration');ensureConfiguration();if(active)configuration?.activate();configuration?.focus();}
 function renderConnection(root,data){
  const connection=connectionStatus(data),count=(data.administration_peers||[]).filter(p=>p.available).length;
  const message=connectionMessage(data);
  renderChanged(root,{connection,count,message,publish:!!data.publish,error:data.status?.error},()=>{
   root.replaceChildren();const row=el('div','relay-connection-summary'),badge=statusBadge(connection.status);badge.dataset.tone=connection.tone;
   row.append(el('strong',null,'Relay server'),badge,el('span','small muted',`${count} ${count===1?'device':'devices'}`));root.append(row);
   if(message){root.append(el('p','route-error',message));const details=el('details','relay-diagnostic');details.append(el('summary',null,'Technical details'),el('p','small muted',data.status.error));root.append(details);}
  });
 }
 async function refreshCatalog(){if(refreshing)return;refreshing=true;refresh.disabled=true;try{await ctx.mutate(()=>ctx.api(api('refresh'),{method:'POST',body:'{}'}));}finally{refreshing=false;refresh.disabled=false;}}
 const refresh=refreshButton('Refresh catalog',refreshCatalog);refresh.hidden=false;views.actions.append(refresh);
 function render(state){current=state;routes.render(state);for(const picker of pickers)picker.draw();const d=state.data||{},status=d.status||{records:[]};
  renderConnection(setup,d);if(configuration)configuration.render(state);else{ensureConfiguration();if(active)configuration?.activate();}
  const providers=(status.records||[]).filter(r=>r.device_id!==d.device_id&&r.publish&&r.services.some(s=>s.kind==='application'));
  const byDevice=new Map((d.administration_peers||[]).filter(p=>p.device_id!==d.device_id).map(peer=>[peer.device_id,{id:peer.device_id,hostname:peer.hostname,peer}]));
  for(const provider of providers){const device=byDevice.get(provider.device_id)||{id:provider.device_id,hostname:provider.hostname};device.provider=provider;byDevice.set(device.id,device);}
  const devices=[...byDevice.values()].sort((a,b)=>a.hostname.localeCompare(b.hostname)||a.id.localeCompare(b.id));
  const connection=connectionStatus(d),needsSetup=connection.status==='Not configured'||connection.status==='Disabled';
  refresh.hidden=needsSetup&&!devices.length;
  catalog.hidden=needsSetup&&!devices.length;

  const remote=!!ctx.ui.state().remote_admin;
  renderChanged(catalog,{devices:devices.map(v=>({id:v.id,hostname:v.hostname,peer:v.peer&&{available:v.peer.available,uuid_url:v.peer.uuid_url,hostname_url:v.peer.hostname_url,conflict:v.peer.conflict},provider:v.provider&&{port:v.provider.port,group:v.provider.group,public_key:v.provider.public_key,services:v.provider.services.filter(s=>s.kind==='application'),expired:v.provider.expires_at*1000<=Date.now()}})),remote,connected:!!status.connected,imports:(state.routes||[]).map(r=>r.editor)},()=>{
   catalog.replaceChildren();
   if(!devices.length){
    const empty=el('section','empty-state empty-state-no-results');
    empty.append(el('p','empty-state-message','No other devices yet.'),el('p','small muted','Connect another Zelport to this relay with the same E2E secret.'));
    catalog.append(empty);return;
   }
   catalog.append(el('h2',null,'Shared services'));
   for(const device of devices){
    const {peer,provider}=device,online=!!status.connected&&(peer?peer.available:provider?.expires_at*1000>Date.now());
    const section=el('section','relay-provider'),header=el('div','relay-device-header'),deviceActions=el('div','relay-device-actions');
    const duplicate=devices.some(other=>other.id!==device.id&&other.hostname===device.hostname);
    const heading=el('h3',null,duplicate?`${device.hostname} · ${device.id.slice(0,8)}`:device.hostname);
    heading.id=`relay-provider-${device.id}`;const badge=statusBadge(online?'Online':'Offline');badge.dataset.tone=online?'ok':'neutral';header.append(heading,badge);
    if(!remote&&peer){
     const address=peer.uuid_url||(!peer.conflict&&peer.hostname_url);
     if(address){const admin=el(online?'a':'span','relay-admin-link','Open administration');if(online){admin.href=address;admin.target='_blank';admin.rel='noopener';admin.setAttribute('aria-label',`Open administration on ${device.hostname}`);}else admin.setAttribute('aria-disabled','true');deviceActions.append(admin);}
    }
    header.append(deviceActions);section.append(header);
    const services=(provider?.services||[]).filter(s=>s.kind==='application');
    if(!services.length){section.append(el('p','small muted',online?'No services shared.':'Device offline.'));catalog.append(section);continue;}
    const unavailable=!status.connected?'Connect to the relay to import services.':!online||provider.expires_at*1000<=Date.now()?'Device offline. Imports resume when it reconnects.':'';
    if(unavailable){const reason=el('p','small muted',unavailable);reason.id=`${heading.id}-availability`;section.append(reason);}
    const add=ctx.ui.adornmentButton(`Import service from ${device.hostname}`,'plus',()=>chooseService(device.id,add));add.className='module-action-icon relay-import-icon';add.disabled=!!unavailable;if(unavailable)add.setAttribute('aria-describedby',`${heading.id}-availability`);deviceActions.append(add);
    const wrap=el('div','relay-services-wrap'),table=el('table','relay-services-table data-table'),head=el('thead'),labels=el('tr'),body=el('tbody');
    table.setAttribute('aria-labelledby',heading.id);
    for(const title of ['Service','Status']){const cell=el('th',null,title);cell.scope='col';labels.append(cell);}
    head.append(labels);table.append(head,body);
    for(const service of services){
     const row=el('tr'),domain=el('td',null,provider.port?`${service.domain}:${provider.port}`:service.domain),statusCell=el('td','small muted');
     const imported=(state.routes||[]).some(r=>r.editor.group===provider.group&&r.editor.device_id===provider.device_id&&r.editor.service_id===service.id);
     domain.dataset.label='Service';statusCell.dataset.label='Status';statusCell.textContent=imported?'Imported':'Not imported';row.append(domain,statusCell);body.append(row);
    }
    wrap.append(table);section.append(wrap);catalog.append(section);
   }
  });
 }
 return {render,openSettings,edit:routes.edit,remove:routes.remove,activate(){active=true;if(!configurationView.hidden)configuration?.activate();},deactivate(){active=false;configuration?.deactivate();},dispose(){active=false;servicePicker?.close();configuration?.dispose();routes.dispose();views.dispose();}};
}
