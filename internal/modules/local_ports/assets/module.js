import {mountConfiguration} from './configuration.js';

const discoveryFailure="Listening ports couldn't be detected. Enter the service port manually.";
const discoverySignature=(data={})=>JSON.stringify([data.discovery_enabled!==false,data.ignored_ports||[],data.ignored_processes||[]]);
const nonLoopbackWarning='This port also listens on non-loopback interfaces.';

// Connections to localhost reach a process bound to the loopback address itself.
// A process that holds the same port only on a wildcard address is shadowed.
const reached=(processes=[])=>{const front=processes.filter(process=>!process.shadowed);return front.length?front:processes;};
const shadowed=(processes=[])=>reached(processes).length<processes.length?processes.filter(process=>process.shadowed):[];
function processName(process){const workload=process.sandbox||process.container;return workload?`${process.name||'Unknown process'} · ${workload}`:process.name||'Unknown process';}
// The warning names the process that exposes the port when it is not the one Zelport reaches.
function exposureWarning(listener){
 const others=shadowed(listener?.processes).filter(process=>process.listens_beyond_loopback);
 const own=reached(listener?.processes).some(process=>process.listens_beyond_loopback);
 return others.length&&!own?`${others.map(processName).join(', ')} also listens on this port on non-loopback interfaces.`:nonLoopbackWarning;
}

function processLabels(all=[]){
 const processes=reached(all);
 if(!processes.length)return {text:'Unknown process',full:'Unknown process',truncated:false};
 let truncated=false;
 const labels=processes.map(process=>{
  const name=process.name||'Unknown process',characters=Array.from(name),shortened=characters.length>20,shortName=shortened?`${characters.slice(0,20).join('')}…`:name;
  truncated ||= shortened;
  // A port of a sandbox or container proxy reads "<process> · <workload>", e.g. "gvproxy · web".
  const workload=process.sandbox||process.container;
  return workload?{text:`${shortName} · ${workload}`,full:`${name} · ${workload}`}:{text:shortName,full:name};
 });
 return {text:labels.map(label=>label.text).join(', '),full:labels.map(label=>label.full).join(', '),truncated};
}

function localPortDescription(all=[]){
 const others=shadowed(all),also=others.length?`\nAlso on this port: ${others.map(processName).join(', ')}`:'';
 return describeProcesses(reached(all))+also;
}
function describeProcesses(processes=[]){
 const pid=()=>processes.map(process=>process.pid).filter(Boolean).join(', ')||'Unknown';
 for(const [key,kind,one,many] of [['sandbox','Docker Sandbox','Sandbox','Sandboxes'],['container','Container','Container','Containers']]){
  const names=[...new Set(processes.map(process=>process[key]).filter(Boolean))];
  if(names.length)return `Kind: ${kind}\n${names.length>1?many:one}: ${names.join(', ')}\nProcess: ${[...new Set(processes.map(process=>process.name).filter(Boolean))].join(', ')}\nPID: ${pid()}`;
 }
 const plural=processes.length>1,names=processes.length?processes.map(process=>process.name||'Unknown process').join(', '):'Unknown process',pids=processes.map(process=>process.pid).filter(Boolean);
 return `Kind: Generic process\n${plural?'Names':'Name'}: ${names}\n${pids.length>1?'PIDs':'PID'}: ${pids.length?pids.join(', '):'Unknown'}`;
}
function processIndicator(ui,listener){
 if(!listener.processes?.length)return null;
 return ui.localPortIndicator(localPortDescription(listener.processes)).root;
}

// The port field is also the filter: typing digits or a process name narrows the
// detected ports below it; Use fills the field and shows the whole list again.
export function listenerPicker(ctx,port,{container=port.wrap,address=null}={}){
 const {adornmentButton,choiceList,choiceRow,choiceState,el,warningIndicator}=ctx.ui;
 const menu=choiceList('Listening ports');menu.className+=' listener-picker-menu';container.append(menu);

 let loading=false,disposed=false,generation=0,request=null,listeners=[],renderError='';
 let configuration=ctx.ui.state?.()?.states?.[ctx.moduleId]?.data||{},configurationKey=discoverySignature(configuration),enabled=configuration.discovery_enabled!==false;
 const refresh=adornmentButton('Refresh listening ports','rotate',()=>refreshInventory());refresh.className+=' listener-picker-refresh';
 const tools=el('div','input-adornment-actions');tools.append(refresh);(port.control||port.wrap).append(tools);
 port.input.setAttribute('aria-controls',menu.id);
 let options=[],query='',filling=false;
 // A new service takes its name from the process of the picked port while the address is still the suggestion.
 let suggestedName=address?.value||'';
 const count=el('span','listener-picker-count');if(tools.prepend)tools.prepend(count);else tools.append(count);
 function usedPorts(){const used=new Map();for(const route of ctx.ui.state?.()?.states?.[ctx.moduleId]?.routes||[]){const value=Number(route.editor?.aux_port);if(value>0&&!used.has(value))used.set(value,route.domain);}return used;}
 function freeName(base){
  const clean=String(base||'').toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'').slice(0,50);if(!clean)return '';
  const taken=new Set(Object.values(ctx.ui.state?.()?.states||{}).flatMap(s=>(s.routes||[]).map(r=>r.domain)));
  let name=clean,n=2;while(taken.has(`${name}.localhost`))name=`${clean}-${n++}`;return name;
 }

 function clearRows(){menu.replaceChildren();options=[];}
 function state(message,{spinner=false,warning=false}={}){
  const row=choiceState(message,{warning});row.className+=' listener-picker-state';row.children[0].className+=' listener-picker-message';
  if(spinner){row.setAttribute('aria-label','Loading listening ports');const indicator=el('span','boot-spinner');indicator.setAttribute('aria-hidden','true');row.replaceChildren(indicator);}
  menu.append(row);
 }
 function usePort(value){
  query='';filling=true;port.input.value=String(value);port.input.dispatchEvent(new Event('input',{bubbles:true}));filling=false;
  const listener=listeners.find(item=>Number(item.port)===Number(value)),name=listener?freeName(reached(listener.processes)[0]?.sandbox||reached(listener.processes)[0]?.container||reached(listener.processes)[0]?.name):'';
  if(address&&name&&address.value===suggestedName){suggestedName=name;address.value=name;address.dispatchEvent(new Event('input',{bubbles:true}));}
  render();port.input.focus();
 }
 function updateSelection(){for(const option of options)option.select(Number(String(port.input.value).trim())===option.value);}
 function render(rows=listeners,error=renderError){
  const scroll=menu.scrollTop;clearRows();tools.hidden=!enabled;refresh.disabled=loading||!enabled;refresh.setAttribute('aria-busy',String(loading));
  // Free ports first, then ports a service already uses (still selectable); each group by number.
  const used=usedPorts(),filtered=rows.filter(listener=>`${listener.port} ${processLabels(listener.processes).full}`.toLowerCase().includes(query)).sort((a,b)=>(used.has(Number(a.port))-used.has(Number(b.port)))||Number(a.port)-Number(b.port));
  ctx.ui.setText(count,query&&enabled&&rows.length?`${filtered.length} of ${rows.length}`:'');
  if(!enabled)state('Port detection is off.');
  else if(loading&&!rows.length)state('',{spinner:true});
  else if(error)state(discoveryFailure,{warning:true});
  else if(!rows.length)state('No listening ports found.');
  else if(!filtered.length&&!/^\d+$/.test(query))state('No detected port matches.');
  for(const listener of enabled?filtered:[]){
   const process=processLabels(listener.processes).full,owner=used.get(Number(listener.port)),trailing=[...(listener.listens_beyond_loopback?[warningIndicator(exposureWarning(listener)).root]:[]),...(owner?[el('span','listener-used-by',owner)]:[])];
   const choice=choiceRow({primary:String(listener.port),secondary:process,label:`Use port ${listener.port}: ${process}${owner?`, used by ${owner}`:''}`,onSelect:()=>usePort(listener.port),trailing});if(owner)choice.row.dataset.used='true';
   choice.row.className+=' listener-option';choice.row.dataset.port=String(listener.port);choice.row.setAttribute('role','listitem');choice.main.className+=' listener-use-port';choice.main.children[0].className+=' listener-port';if(choice.main.children[1])choice.main.children[1].className+=' listener-process';
   menu.append(choice.row);options.push({...choice,value:Number(listener.port)});
  }
  // A port that is not listening yet (a service will start later) can still be used.
  const typedPort=/^\d+$/.test(query)?Number(query):0;
  if(enabled&&typedPort>=1&&typedPort<=65535&&!rows.some(listener=>Number(listener.port)===typedPort)){
   const choice=choiceRow({primary:`Use port ${typedPort}`,label:`Use port ${typedPort}`,onSelect:()=>usePort(typedPort)});choice.row.className+=' listener-manual';choice.row.dataset.port=String(typedPort);choice.row.setAttribute('role','listitem');menu.append(choice.row);options.push({...choice,value:typedPort});
  }
  updateSelection();menu.scrollTop=scroll;
 }
 async function load(){
  if(disposed||loading)return;if(!enabled){render();return;}
  request?.abort();const controller=new AbortController();request=controller;const current=++generation;loading=true;render([]);
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(disposed||current!==generation)return;loading=false;enabled=result.discovery_enabled!==false;listeners=Array.isArray(result.listeners)?result.listeners:[];renderError='';render();
  }catch(error){
   if(disposed||current!==generation||controller.signal.aborted)return;
   loading=false;listeners=[];renderError=discoveryFailure;render();
  }
 }
 async function refreshInventory(){
  if(disposed||loading)return;if(!enabled){render();return;}
  request?.abort();const controller=new AbortController();request=controller;const current=++generation;loading=true;render();
  try{
   await ctx.api(`/api/modules/${ctx.moduleId}/listeners/refresh`,{method:'POST',signal:controller.signal});
   if(disposed||current!==generation)return;
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(disposed||current!==generation)return;
   enabled=result.discovery_enabled!==false;listeners=Array.isArray(result.listeners)?result.listeners:[];renderError='';loading=false;render();
  }catch(error){
   if(disposed||current!==generation||controller.signal.aborted)return;
   loading=false;renderError=discoveryFailure;render();
  }
 }
 const typed=()=>{if(filling)return;query=port.input.value.trim().toLowerCase();render();};port.input.addEventListener('input',typed);
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;generation++;request?.abort();port.input.removeEventListener('input',typed);ctx.signal?.removeEventListener('abort',abort);}
 function configure(data={}){
  const key=discoverySignature(data),nextEnabled=data.discovery_enabled!==false;if(disposed||(key===configurationKey&&enabled===nextEnabled))return;
  configurationKey=key;enabled=nextEnabled;generation++;request?.abort();request=null;loading=false;listeners=[];renderError='';if(enabled)load();else render();
 }
 load();
 return {menu,refresh,configure,dispose};
}

export function mount(ctx){
 const {adornedField,el,noListenerIndicator,serviceModule,warningIndicator}=ctx.ui;
 let latest={routes:[],data:{}},visible=false,disposed=false,request=null,routePorts='',discoveryKey='',pickers=new Set(),inventory={health:'unknown',byPort:new Map()};
 const views=ctx.ui.moduleViews({...ctx,moduleName:ctx.moduleName||'Local Ports'},{onChange(view){if(view==='configuration')settings.open();}});
 const settings=mountConfiguration(ctx,views.configuration);
 const view=serviceModule({...ctx,panel:views.overview,toolbar:views.actions},{
  routeMenu:true,
  columns:['Service','Service port','Info','Actions'],
  tableLabel:'Local Ports services',
  serviceLabel:(route,state)=>`${route.domain}:${state.port}`,
  targetLabel:route=>String(route.editor?.aux_port||''),
  rowState(s){return (s.routes||[]).map(route=>{
   const listener=inventory.byPort.get(Number(route.editor?.aux_port));
   if(!listener)return [route.id,inventory.health==='ready'?'missing':'unverified'];
   return [route.id,!!listener.listens_beyond_loopback,localPortDescription(listener.processes)];
  });},
  rowIndicators(route){
   const port=Number(route.editor?.aux_port),listener=inventory.byPort.get(port);
   if(!listener)return inventory.health==='ready'&&port>0?[noListenerIndicator(`No listener currently detected on port ${port}.`).root]:[];
   const indicators=[];
   if(listener.listens_beyond_loopback)indicators.push(warningIndicator(exposureWarning(listener)).root);
   const source=processIndicator(ctx.ui,listener);if(source)indicators.push(source);
   return indicators;
  },
  emptyState:{kind:'not-configured',message:'No local services yet. Add a domain and the port where your service listens on this computer.',actionLabel:'+ Add service'},
  fields(s,group,state,{address,isNew}={}){
   const element=el('div','service-fields-fill'),port=adornedField('Port',s.aux_port?String(s.aux_port):isNew===false?'3000':'');port.input.required=true;port.input.autocomplete='off';port.input.spellcheck=false;port.input.placeholder='Port number or process name';
   port.control.prepend(ctx.ui.icon('search',{className:'input-adornment-icon'}));element.append(port.wrap);
   const picker=listenerPicker(ctx,port,{container:element,address:isNew?address:null});pickers.add(picker);
   return {element,read(){const value=port.input.value.trim(),number=Number(value);if(!/^\d+$/.test(value)||number<1||number>65535){const error=Error('Enter a port number or pick one below.');error.input=port.input;throw error;}return {aux_port:number};},dispose(){pickers.delete(picker);picker.dispose();}};
  }
 });
 async function loadInventory(){
  if(!visible||disposed||request||latest.data?.discovery_enabled===false||!latest.routes?.length)return;
  const controller=new AbortController();request=controller;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:result.discovery_enabled===false?'disabled':'ready',checkedAt:new Date(),byPort:new Map((result.configured_listeners||result.listeners||[]).map(listener=>[Number(listener.port),listener]))};
   view.render(latest);ctx.statusChanged?.();
  }catch{
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:'unavailable',byPort:new Map()};view.render(latest);ctx.statusChanged?.();
  }finally{if(request===controller)request=null;}
 }
 function stop(){visible=false;request?.abort();request=null;}
 return {
  ...view,
  // Listeners are read when a page that shows them opens and on explicit Refresh only.
  refreshRouteStatus(){if(!visible||disposed)return;request?.abort();request=null;loadInventory();},
  routeTarget(route){
   const port=Number(route.editor?.aux_port),listener=inventory.health==='ready'?inventory.byPort.get(port):null;
   return port>0?`Port ${port}${listener?.processes?.length?` · ${processLabels(listener.processes).text}`:''}`:'';
  },
  routeStatus(route){
   const port=Number(route.editor?.aux_port);if(inventory.health!=='ready'||port<=0)return {tone:'neutral',label:'Not checked'};
   const listener=inventory.byPort.get(port);
   const checked=inventory.checkedAt?` Checked at ${inventory.checkedAt.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})}.`:'';
   if(!listener)return {tone:'warning',label:'No listener',detail:`Nothing listens on port ${port}.${checked}`};
   return {tone:'ok',label:'Listening',detail:checked.trim()};
  },
  routeWarning(route){
   const port=Number(route.editor?.aux_port);if(inventory.health!=='ready'||port<=0)return '';
   const listener=inventory.byPort.get(port);if(!listener?.listens_beyond_loopback)return '';
   const checked=inventory.checkedAt?` Checked at ${inventory.checkedAt.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})}.`:'';
   return `${exposureWarning(listener)}${checked}`;
  },
  routeNotice(route){
   const port=Number(route.editor?.aux_port);if(inventory.health!=='ready'||port<=0)return '';
   const listener=inventory.byPort.get(port);
   return !listener?`Last detected: no listener on port ${port}.`:listener.listens_beyond_loopback?'Last detected: this port also listens on non-loopback interfaces.':'';
  },

  openSettings(){views.show('configuration');settings.open();},
  render(s){
   latest=s;settings.render(s);settings.open();const key=discoverySignature(s.data),ports=JSON.stringify((s.routes||[]).map(route=>route.editor?.aux_port));
   const settingsChanged=key!==discoveryKey,portsChanged=ports!==routePorts;
   if(settingsChanged||portsChanged){
    discoveryKey=key;routePorts=ports;request?.abort();request=null;inventory={health:'unknown',byPort:new Map()};
    if(settingsChanged)for(const picker of pickers)picker.configure(s.data);
    if(visible)loadInventory();
   }
   view.render(s);
  },
  activate(){if(visible)return;visible=true;routePorts=JSON.stringify((latest.routes||[]).map(route=>route.editor?.aux_port));view.render(latest);loadInventory();view.activate();},
  deactivate(){stop();view.deactivate();},
  dispose(){disposed=true;stop();for(const picker of pickers)picker.dispose();pickers.clear();settings.dispose();view.dispose();views.dispose();}
 };
}
