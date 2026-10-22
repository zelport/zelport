const discoveryFailure="Listening ports couldn't be detected. Enter the Aux port manually.";
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
 const {adornmentButton,el,textTooltip,warningIndicator}=ctx.ui;
 const menu=el('section','listener-picker-menu');
 menu.setAttribute('role','list');menu.setAttribute('aria-label','Listening ports');port.wrap.append(menu);

 let loading=false,disposed=false,generation=0,request=null,listeners=[],renderError='';
 const heading=el('div','listener-picker-header'),title=el('strong',null,'Detected listening ports');
 const refresh=adornmentButton('Refresh listening ports','rotate',()=>refreshInventory());refresh.className+=' listener-picker-refresh';
 heading.append(title,refresh);menu.append(heading);

 function clearRows(){for(const child of [...menu.children])if(child!==heading)child.remove();}
 function state(message,{spinner=false,warning=false}={}){
  const row=el('div',`listener-picker-state${warning?' listener-picker-state-warning':''}`);row.disabled=true;row.setAttribute('role','status');
  if(spinner){row.setAttribute('aria-label','Loading listening ports');const indicator=el('span','boot-spinner');indicator.setAttribute('aria-hidden','true');row.append(indicator);}
  else row.append(el('span','listener-picker-message',message));
  menu.append(row);
 }
 function usePort(value){
  port.input.value=String(value);port.input.dispatchEvent(new Event('input',{bubbles:true}));port.input.focus();
 }
 function render(rows=listeners,error=renderError){
  const scroll=menu.scrollTop;clearRows();refresh.disabled=loading;refresh.setAttribute('aria-busy',String(loading));
  if(loading&&!rows.length)state('',{spinner:true});
  else if(error)state(discoveryFailure,{warning:true});
  else if(!rows.length)state('No listening ports found.');
  else for(const listener of rows){
   const process=processLabels(listener.processes),row=el('div','listener-option'),actions=el('div','listener-option-actions'),processNode=process.truncated?textTooltip(process.text,process.full,'listener-process').root:el('span','listener-process',process.text);row.dataset.port=String(listener.port);row.setAttribute('role','listitem');row.setAttribute('aria-label',`Port ${listener.port}: ${process.full}`);
   row.append(el('strong','listener-port',String(listener.port)),processNode);
   if(listener.listens_beyond_loopback)actions.append(warningIndicator(nonLoopbackWarning).root);
   const source=processIndicator(ctx.ui,listener);if(source)actions.append(source);
   const use=adornmentButton(`Choose port ${listener.port}`,'arrowUp',()=>usePort(listener.port),{tooltipClass:'choose-port-tooltip'});use.className+=' listener-use-port';actions.append(use);row.append(actions);menu.append(row);
  }
  menu.scrollTop=scroll;
 }
 async function load(){
  if(disposed||loading)return;
  request?.abort();const controller=new AbortController();request=controller;const current=++generation;loading=true;render([]);
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(disposed||current!==generation)return;loading=false;listeners=Array.isArray(result.listeners)?result.listeners:[];renderError='';render();
  }catch(error){
   if(disposed||current!==generation||controller.signal.aborted)return;
   loading=false;listeners=[];renderError=discoveryFailure;render();
  }
 }
 async function refreshInventory(){
  if(disposed||loading)return;
  request?.abort();const controller=new AbortController();request=controller;const current=++generation;loading=true;render();
  try{
   await ctx.api(`/api/modules/${ctx.moduleId}/listeners/refresh`,{method:'POST',signal:controller.signal});
   if(disposed||current!==generation)return;
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(disposed||current!==generation)return;
   listeners=Array.isArray(result.listeners)?result.listeners:[];renderError='';loading=false;render();
  }catch(error){
   if(disposed||current!==generation||controller.signal.aborted)return;
   loading=false;if(!listeners.length)renderError=discoveryFailure;render();
  }
 }
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;generation++;request?.abort();ctx.signal?.removeEventListener('abort',abort);}
 load();
 return {menu,refresh,dispose};
}

export function mount(ctx){
 const {el,noListenerIndicator,numberStepperField,serviceModule,warningIndicator}=ctx.ui;
 let latest={routes:[],data:{}},visible=false,disposed=false,request=null,timer=null,routePorts='',inventory={health:'unknown',byPort:new Map()};
 const view=serviceModule(ctx,{
  routeMenu:true,
  columns:['Service','Local (Aux) port','Info','Actions'],
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
  emptyState:{kind:'not-configured',message:'No local ports yet. Add a domain and an Aux port.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),port=numberStepperField('Local (Aux) port',s.aux_port||3000,{min:1,max:65535});port.input.required=true;
   const picker=listenerPicker(ctx,port);element.append(port.wrap);
   return {element,read:()=>({aux_port:Number(port.input.value)}),dispose:picker.dispose};
  }
 });
 function schedule(){if(visible&&!disposed&&latest.routes?.length)timer=setTimeout(loadInventory,15000);}
 async function loadInventory(){
  if(!visible||disposed||request||!latest.routes?.length)return;
  clearTimeout(timer);timer=null;const controller=new AbortController();request=controller;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:controller.signal});
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:'ready',byPort:new Map((Array.isArray(result.listeners)?result.listeners:[]).map(listener=>[Number(listener.port),listener]))};
   view.render(latest);
  }catch{
   if(!visible||disposed||controller.signal.aborted)return;
   inventory={health:'unavailable',byPort:new Map()};view.render(latest);
  }finally{if(request===controller){request=null;schedule();}}
 }
 function stop(){visible=false;clearTimeout(timer);timer=null;request?.abort();request=null;inventory={health:'unknown',byPort:new Map()};}
 return {
  ...view,
  render(s){latest=s;view.render(s);const ports=JSON.stringify((s.routes||[]).map(route=>route.editor?.aux_port));if(visible&&ports!==routePorts){routePorts=ports;loadInventory();}},
  activate(){if(visible)return;visible=true;routePorts=JSON.stringify((latest.routes||[]).map(route=>route.editor?.aux_port));view.render(latest);loadInventory();view.activate();},
  deactivate(){stop();view.deactivate();},
  dispose(){disposed=true;stop();view.dispose();}
 };
}
