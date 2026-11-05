import {mountConfiguration} from './configuration.js';
function connectionStatus(data) {
 if(!data.relay_url?.trim())return {status:'Not configured',tone:'neutral'};
 if(!data.enabled)return {status:'Disabled',tone:'neutral'};
 const status=data.status||{};
 return status.connected?{status:'Connected',tone:'ok'}:status.error?{status:'Connection failed',tone:'error'}:{status:'Connecting',tone:'pending'};
}
// The status line on the Relay page uses the service status vocabulary: Off, Connecting, Connected, Unreachable.
export function relayStatusLine(data){
 const connection=connectionStatus(data);let host='';try{host=data.relay_url?new URL(data.relay_url).host:'';}catch{}
 if(connection.status==='Not configured'||connection.status==='Disabled')return {host,tone:'neutral',label:'Off'};
 if(connection.tone==='ok')return {host,tone:'ok',label:'Connected'};
 if(connection.tone==='error')return {host,tone:'error',label:'Unreachable',detail:[connectionMessage(data),data.status?.error].filter(Boolean).join(' ')};
 return {host,tone:'neutral',label:'Connecting'};
}
// "2 of 6 imported": imported services are part of what a linked Zelport shares.
export function importSummary({online,shared,imported}){return online?`${imported} of ${shared} imported`:`Offline · ${imported} imported`;}
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
 const {el,button,field,serviceModule,renderChanged,statusBadge,refreshButton}=ctx.ui,ui=ctx.ui;
 let current,configuration,servicePicker=null,active=false,refreshing=false,expandedInitialized=false;const expanded=new Set();
 const overviewView=el('section','relay-services-view'),configurationView=el('section','relay-configuration-view');
 const routePanel=el('section'),setup=el('section','relay-setup'),catalog=el('section','relay-catalog'),settingsPanel=el('section','relay-settings-panel');
 // Imports are listed on Services; routePanel only hosts the shared import form state.
 routePanel.hidden=true;overviewView.append(routePanel,catalog);
 configurationView.append(settingsPanel);configurationView.hidden=true;
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
  emptyState:{kind:'no-results',message:'No services imported yet.'},fields(settings,group,state,{address,setReady}={}){
  if(!settings.device_id)return sharedServicePicker(address,setReady);
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
 // The import keeps the shared name; when it is taken, the Zelport's name tells them apart (grafana-build-server).
 function importName(service,provider){
  const used=new Set(Object.values(ctx.ui.state().states||{}).flatMap(s=>(s.routes||[]).map(r=>r.domain)));for(const route of current?.routes||[])used.add(route.domain);
  const base=service.domain.replace(/\.localhost$/,''),host=String(provider?.hostname||'').toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'');
  if(!used.has(`${base}.localhost`))return base;if(host&&!used.has(`${base}-${host}.localhost`))return `${base}-${host}`;
  let n=2;while(used.has(`${base}-${n}.localhost`))n++;return `${base}-${n}`;
 }
 function sharedServicePicker(address,setReady=()=>{}){
  const {choiceGroup,choiceList,choiceRow,choiceState,optionSwitch,serviceOptions}=ctx.ui;
  const element=el('div','service-fields-fill'),list=choiceList('Shared services'),options=serviceOptions(),origin=optionSwitch('Use target origin',false,originHelp);
  options.append(origin.row);element.append(fieldLabel('Shared service'),list,options);
  let selected=null,drawn='',suggested=address?.value||'';const rows=[];
  const tab=ctx.ui.state().modules?.find(m=>m.id===ctx.moduleId)?.tab_id;
  const settingsLink=()=>{if(!tab)return undefined;const link=el('a','choice-state-link','Relay settings');link.href=`#${tab}`;return link;};
  function pick(entry){selected=entry;setReady(true);for(const row of rows)row.select(row.key===entry.key);
   if(address&&address.value===suggested){suggested=importName(entry.service,entry.provider);address.value=suggested;address.dispatchEvent(new Event('input',{bubbles:true}));}}
  function draw(){
   const d=current?.data||{},status=d.status||{},connected=!!status.connected,providers=(status.records||[]).filter(r=>r.device_id!==d.device_id&&r.publish);
   const peers=(d.administration_peers||[]).filter(p=>p.device_id!==d.device_id&&!providers.some(r=>r.device_id===p.device_id));
   const configured=!!d.relay_url?.trim()&&d.enabled,key=JSON.stringify([configured,connected,providers.map(r=>[r.device_id,r.hostname,r.port,r.expires_at*1000>Date.now(),r.services.filter(s=>s.kind==='application').map(s=>[s.id,s.domain])]),peers.map(p=>[p.device_id,p.hostname,p.available])]);
   if(key===drawn)return;drawn=key;const focused=list.contains(document.activeElement)?document.activeElement.dataset?.key:null;
   list.replaceChildren();rows.length=0;let keepSelection=false;
   const imports=current?.routes||[];
   const badge=(online,device,services)=>el('span','relay-group-count',importSummary({online,shared:services.length,imported:imports.filter(r=>r.editor.device_id===device.device_id).length}));
   const names=[...providers,...peers].map(device=>device.hostname),deviceName=device=>names.filter(name=>name===device.hostname).length>1?`${device.hostname} · ${device.device_id.slice(0,8)}`:device.hostname;
   for(const provider of [...providers].sort((a,b)=>a.hostname.localeCompare(b.hostname))){
    const online=connected&&provider.expires_at*1000>Date.now(),services=provider.services.filter(s=>s.kind==='application');
    list.append(choiceGroup(deviceName(provider),badge(online,provider,services)));
    if(!online)continue;
    if(!services.length)list.append(choiceState('No services shared.'));
    for(const service of [...services].sort((a,b)=>a.domain.localeCompare(b.domain))){
     const key=`${provider.device_id}/${service.id}`,entry={key,provider,service},name=service.domain.replace(/\.localhost$/,'');
     // A service already imported cannot be imported twice; its row shows the local address instead.
     const existing=imports.find(r=>r.editor.group===provider.group&&r.editor.device_id===provider.device_id&&r.editor.service_id===service.id);
     const choice=choiceRow({primary:name,label:existing?`${name} from ${deviceName(provider)}, imported as ${existing.domain}`:`Import ${name} from ${deviceName(provider)}`,onSelect:()=>pick(entry),disabled:!!existing,trailing:existing?[el('span','relay-imported-as',existing.domain)]:[]});
     if(selected?.key===key&&online){keepSelection=true;selected=entry;}
     choice.row.className+=' relay-shared-option';choice.main.dataset.key=key;choice.select(selected?.key===key);list.append(choice.row);rows.push({...choice,key});
    }
   }
   for(const peer of peers){list.append(choiceGroup(deviceName(peer),badge(!!(connected&&peer.available),peer,[])));if(connected&&peer.available)list.append(choiceState('No services shared.'));}
   const listed=providers.length+peers.length>0;
   if(!listed)list.append(choiceState(!configured?'The relay is not set up.':!connected?'Not connected to the relay.':'No other devices yet.',{action:settingsLink()}));
   else if(!connected)list.prepend(choiceState('Not connected to the relay.',{action:settingsLink()}));
   if(!keepSelection)selected=null;setReady(!!selected);options.hidden=!listed;
   if(focused)[...list.querySelectorAll('button')].find(button=>button.dataset?.key===focused)?.focus();
  }
  draw();const picker={draw};pickers.add(picker);
  return {element,read(){if(!selected)throw Error('Choose a service to import.');const {provider,service}=selected;return {group:provider.group,device_id:provider.device_id,service_id:service.id,provider_key:provider.public_key,rewrite_origin:origin.input.checked};},dispose(){pickers.delete(picker);}};
 }
 function importService(provider,service){
  const domain=`${importName(service,provider)}.localhost`;
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
 const statusIndicator=ctx.ui.serviceStatus(),statusDot=el('span','relay-linked-dot'),statusHost=el('strong','relay-status-host');
 function renderConnection(data){
  const root=configuration?.status;if(!root)return;
  if(!root.children.length)root.append(statusDot,statusHost,statusIndicator.root);
  const line=relayStatusLine(data);statusIndicator.update(line);statusDot.dataset.tone=line.tone;ui.setText(statusHost,line.host);statusHost.hidden=!line.host;
 }
 async function refreshCatalog(){if(refreshing)return;refreshing=true;refresh.disabled=true;try{await ctx.mutate(()=>ctx.api(api('refresh'),{method:'POST',body:'{}'}));}finally{refreshing=false;refresh.disabled=false;}}
 // The catalog follows the relay through the regular state refresh; no separate refresh control.
 const refresh=refreshButton('Refresh catalog',refreshCatalog);refresh.hidden=true;
 function render(state){current=state;routes.render(state);for(const picker of pickers)picker.draw();const d=state.data||{},status=d.status||{records:[]};
  if(configuration)configuration.render(state);else{ensureConfiguration();if(active)configuration?.activate();}renderConnection(d);
  const providers=(status.records||[]).filter(r=>r.device_id!==d.device_id&&r.publish&&r.services.some(s=>s.kind==='application'));
  const byDevice=new Map((d.administration_peers||[]).filter(p=>p.device_id!==d.device_id).map(peer=>[peer.device_id,{id:peer.device_id,hostname:peer.hostname,peer}]));
  for(const provider of providers){const device=byDevice.get(provider.device_id)||{id:provider.device_id,hostname:provider.hostname};device.provider=provider;byDevice.set(device.id,device);}
  const devices=[...byDevice.values()].sort((a,b)=>a.hostname.localeCompare(b.hostname)||a.id.localeCompare(b.id));
  const connection=connectionStatus(d),needsSetup=connection.status==='Not configured'||connection.status==='Disabled';
  if(!expandedInitialized&&devices.length){expandedInitialized=true;if(devices.length===1)expanded.add(devices[0].id);}
  refresh.hidden=needsSetup&&!devices.length;
  catalog.hidden=needsSetup&&!devices.length;

  // Linked Zelports sit between the imports and Configuration.
  const heading=[...(ctx.panel.children||[])].find(node=>node.classList?.contains('source-section-title'));
  if(heading&&catalog.nextSibling!==heading)ctx.panel.insertBefore(catalog,heading);
  // Until relay is set up the page shows only its configuration.
  const services=[...(ctx.panel.children||[])].find(node=>node.classList?.contains('source-services'));
  const imports=state.routes||[],port=ctx.ui.state().port;
  const view=devices.map(device=>{
   const {peer,provider}=device,online=!!status.connected&&(peer?peer.available:provider?.expires_at*1000>Date.now());
   const shared=(provider?.services||[]).filter(s=>s.kind==='application').map(service=>{const route=imports.find(r=>r.editor.group===provider.group&&r.editor.device_id===provider.device_id&&r.editor.service_id===service.id);return {id:service.id,name:service.domain.replace(/\.localhost$/,''),domain:service.domain,imported:route?.domain||''};}).sort((a,b)=>a.name.localeCompare(b.name));
   const duplicate=devices.some(other=>other.id!==device.id&&other.hostname===device.hostname);
   return {id:device.id,name:duplicate?`${device.hostname} · ${device.id.slice(0,8)}`:device.hostname,online,shared,imported:imports.filter(r=>r.editor.device_id===device.id).length};
  });
  renderChanged(catalog,{view,port,expanded:[...expanded],needsSetup},()=>{
   catalog.replaceChildren();catalog.hidden=needsSetup&&!devices.length;if(catalog.hidden)return;
   catalog.append(el('h2','source-section-title','Linked Zelports'));
   const card=el('div','relay-linked');catalog.append(card);
   if(!view.length){card.append(el('p','relay-linked-empty',status.connected?'No linked Zelports':'Not connected'));return;}
   for(const device of view){
    const open=device.online&&expanded.has(device.id),row=el(device.online?'button':'div','relay-linked-row'),chevron=el('span','relay-chevron'),dot=el('span','relay-linked-dot'),name=el('strong',null,device.name);
    dot.dataset.tone=device.online?'ok':'off';
    row.append(chevron,dot,name,el('span','relay-linked-count',importSummary({online:device.online,shared:device.shared.length,imported:device.imported})));
    if(device.online){row.type='button';row.setAttribute('aria-expanded',String(open));chevron.append(ctx.ui.icon(open?'chevronDown':'chevronRight',{className:'relay-chevron-icon'}));row.addEventListener('click',()=>{if(expanded.has(device.id))expanded.delete(device.id);else expanded.add(device.id);render(current);});}
    const group=el('section','relay-linked-device');group.append(row);
    if(open){
     const list=el('div','relay-linked-services');list.setAttribute('role','list');
     for(const service of device.shared){
      const item=el('div','relay-linked-service');item.setAttribute('role','listitem');item.append(el('span','relay-linked-service-name',service.name));
      if(service.imported){const link=el('a','relay-linked-import',`${service.imported}:${port}`);link.href=`http://${service.imported}:${port}`;link.target='_blank';link.rel='noopener';item.append(link);}
      else{const add=button('Import',()=>{const provider=current.data?.status?.records?.find(r=>r.device_id===device.id);const shared=provider?.services?.find(s=>s.id===service.id);if(provider&&shared)importService(provider,shared);},'secondary relay-linked-import-button');add.setAttribute('aria-label',`Import ${service.name} from ${device.name}`);item.append(add);}
      list.append(item);
     }
     if(!device.shared.length)list.append(el('p','relay-linked-empty','No shared services'));
     group.append(list);
    }
    card.append(group);
   }
  });
 }
 function routeTarget(route){
  const record=current?.data?.status?.records?.find(r=>r.device_id===route.editor?.device_id),service=record?.services?.find(s=>s.id===route.editor?.service_id);
  const name=service?.domain?.replace(/\.localhost$/,'');return name?`${providerName(route)} · ${name}`:providerName(route);
 }
 // Until relay is set up there is nothing to import: the list says so and Add service is not offered.
 function relayReady(){const connection=connectionStatus(current?.data||{});return connection.status!=='Not configured'&&connection.status!=='Disabled';}
 return {render,routeTarget,emptyLabel:()=>relayReady()?'':'Configure relay first',canAdd:()=>relayReady(),openSettings,edit:routes.edit,remove:routes.remove,activate(){active=true;if(!configurationView.hidden)configuration?.activate();},deactivate(){active=false;configuration?.deactivate();},dispose(){active=false;servicePicker?.close();configuration?.dispose();routes.dispose();views.dispose();}};
}
