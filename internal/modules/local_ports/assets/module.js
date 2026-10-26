import {mountConfiguration} from './configuration.js';

const discoveryFailure="Listening ports couldn't be detected. Enter the service port manually.";
const discoverySignature=(data={})=>JSON.stringify([data.discovery_enabled!==false,data.ignored_ports||[],data.ignored_processes||[]]);
const nonLoopbackWarning='This port also listens on non-loopback interfaces.';

function processLabels(processes=[]){
 if(!processes.length)return {text:'Unknown process',full:'Unknown process',truncated:false};
 let truncated=false;
 const labels=processes.map(process=>{
  const name=process.name||'Unknown process',characters=Array.from(name),shortened=characters.length>20,shortName=shortened?`${characters.slice(0,20).join('')}…`:name;
  truncated ||= shortened;
  return {text:shortName,full:name};
 });
 return {text:labels.map(label=>label.text).join(', '),full:labels.map(label=>label.full).join(', '),truncated};
}

function localPortDescription(processes=[]){
 const plural=processes.length>1,names=processes.length?processes.map(process=>process.name||'Unknown process').join(', '):'Unknown process',pids=processes.map(process=>process.pid).filter(Boolean);
 return `Kind: Generic process\n${plural?'Names':'Name'}: ${names}\n${pids.length>1?'PIDs':'PID'}: ${pids.length?pids.join(', '):'Unknown'}`;
}
function processIndicator(ui,listener){
 if(!listener.processes?.length)return null;
 return ui.localPortIndicator(localPortDescription(listener.processes)).root;
}

export function listenerPicker(ctx,port){
 const {adornmentButton,button,el,field,textTooltip,warningIndicator}=ctx.ui;
 const menu=el('section','listener-picker-menu');
 menu.setAttribute('role','list');menu.setAttribute('aria-label','Listening ports');port.wrap.append(menu);

 let loading=false,disposed=false,generation=0,request=null,listeners=[],renderError='';
 let configuration=ctx.ui.state?.()?.states?.[ctx.moduleId]?.data||{},configurationKey=discoverySignature(configuration),enabled=configuration.discovery_enabled!==false;
 const heading=el('div','listener-picker-header'),title=el('strong',null,'Detected listening ports');
 const refresh=adornmentButton('Refresh listening ports','rotate',()=>refreshInventory());refresh.className+=' listener-picker-refresh';
 const search=field('Search listening ports','','search');search.input.className+=' listener-picker-search';search.input.placeholder='Port or process';search.wrap.className+=' listener-picker-filter';search.wrap.children[0].className='visually-hidden';
 heading.className+=' picker-toolbar';heading.append(title,search.wrap,refresh);menu.append(heading);
 let options=[];

 function clearRows(){for(const child of [...menu.children])if(child!==heading&&child!==search.wrap)child.remove();options=[];}
 function state(message,{spinner=false,warning=false}={}){
  const row=el('div',`listener-picker-state${warning?' listener-picker-state-warning':''}`);row.disabled=true;row.setAttribute('role','status');
  if(spinner){row.setAttribute('aria-label','Loading listening ports');const indicator=el('span','boot-spinner');indicator.setAttribute('aria-hidden','true');row.append(indicator);}
  else row.append(el('span','listener-picker-message',message));
  menu.append(row);
 }
 function usePort(value){
  port.input.value=String(value);port.input.dispatchEvent(new Event('input',{bubbles:true}));port.input.focus();
 }
 function updateSelection(){for(const {row,use,value} of options){const selected=Number(port.input.value)===value;row.dataset.selected=String(selected);use.textContent=selected?'Selected':'Use';use.setAttribute('aria-pressed',String(selected));}}
 function render(rows=listeners,error=renderError){
  const scroll=menu.scrollTop;clearRows();heading.hidden=search.wrap.hidden=!enabled;refresh.disabled=loading||!enabled;refresh.setAttribute('aria-busy',String(loading));
  const query=search.input.value.trim().toLowerCase(),filtered=rows.filter(listener=>`${listener.port} ${processLabels(listener.processes).full}`.toLowerCase().includes(query));
  if(!enabled)state('Port detection is off.');
  else if(loading&&!rows.length)state('',{spinner:true});
  else if(error)state(discoveryFailure,{warning:true});
  else if(!rows.length)state('No listening ports found.');
  else if(!filtered.length)state('No matching ports.');
  for(const listener of enabled?filtered:[]){
   const process=processLabels(listener.processes),row=el('div','listener-option'),actions=el('div','listener-option-actions'),processNode=process.truncated?textTooltip(process.text,process.full,'listener-process').root:el('span','listener-process',process.text);row.dataset.port=String(listener.port);row.setAttribute('role','listitem');row.setAttribute('aria-label',`Port ${listener.port}: ${process.full}`);
   row.append(el('strong','listener-port',String(listener.port)),processNode);
   if(listener.listens_beyond_loopback)actions.append(warningIndicator(nonLoopbackWarning).root);
   const source=processIndicator(ctx.ui,listener);if(source)actions.append(source);
   const use=button('Use',()=>usePort(listener.port),'secondary listener-use-port');use.setAttribute('aria-label',`Use port ${listener.port}`);actions.append(use);row.append(actions);menu.append(row);options.push({row,use,value:Number(listener.port)});
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
 const filter=()=>render(),searchKeydown=event=>{if(event.key==='Enter')event.preventDefault();};search.input.addEventListener('input',filter);search.input.addEventListener('keydown',searchKeydown);port.input.addEventListener('input',updateSelection);
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;generation++;request?.abort();search.input.removeEventListener('input',filter);search.input.removeEventListener('keydown',searchKeydown);port.input.removeEventListener('input',updateSelection);ctx.signal?.removeEventListener('abort',abort);}
 function configure(data={}){
  const key=discoverySignature(data),nextEnabled=data.discovery_enabled!==false;if(disposed||(key===configurationKey&&enabled===nextEnabled))return;
  configurationKey=key;enabled=nextEnabled;generation++;request?.abort();request=null;loading=false;listeners=[];renderError='';if(enabled)load();else render();
 }
 load();
 return {menu,refresh,configure,dispose};
}

export function mount(ctx){
 const {el,noListenerIndicator,numberStepperField,serviceModule,warningIndicator}=ctx.ui;
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
   if(listener.listens_beyond_loopback)indicators.push(warningIndicator(nonLoopbackWarning).root);
   const source=processIndicator(ctx.ui,listener);if(source)indicators.push(source);
   return indicators;
  },
  emptyState:{kind:'not-configured',message:'No local services yet. Add a domain and the port where your service listens on this computer.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),port=numberStepperField('Service port',s.aux_port||3000,{min:1,max:65535});port.input.required=true;
   const picker=listenerPicker(ctx,port);pickers.add(picker);element.append(port.wrap,el('p','small muted','The port on this computer.'));
   return {element,read:()=>({aux_port:Number(port.input.value)}),dispose(){pickers.delete(picker);picker.dispose();}};
  }
 });
 async function loadInventory(){
  if(!visible||disposed||request||latest.data?.discovery_enabled===false||!latest.routes?.length)return;
  const controller=new AbortController();request=controller;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:result.discovery_enabled===false?'disabled':'ready',checkedAt:new Date(),byPort:new Map((result.configured_listeners||result.listeners||[]).map(listener=>[Number(listener.port),listener]))};
   view.render(latest);
  }catch{
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:'unavailable',byPort:new Map()};view.render(latest);
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
   const port=Number(route.editor?.aux_port);if(inventory.health!=='ready'||port<=0)return {tone:'neutral',label:'Configured'};
   const listener=inventory.byPort.get(port);
   const checked=inventory.checkedAt?` Checked at ${inventory.checkedAt.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})}.`:'';
   if(!listener)return {tone:'warning',label:'No listener',detail:`Nothing listens on port ${port}.${checked}`};
   if(listener.listens_beyond_loopback)return {tone:'warning',label:'Exposed',detail:`${nonLoopbackWarning}${checked}`};
   return {tone:'ok',label:'Listening',detail:checked.trim()};
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
