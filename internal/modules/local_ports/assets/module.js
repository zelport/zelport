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

function sourceLabel(listener){
 const sandboxes=listener.sandboxes||[];
 if(listener.kind==='docker_sandbox'&&sandboxes.length===1){const sandbox=sandboxes[0];return {process:processLabels([{name:sandbox.name}]),container:null,sandbox};}
 const containers=listener.containers||[];
 if(listener.kind==='container'&&containers.length===1){const container=containers[0];return {process:processLabels([{name:container.name}]),container,sandbox:null};}
 if(listener.kind==='unknown')return {process:processLabels(listener.processes),container:null,sandbox:null,unknown:true};
 if(listener.kind==='local_process')return {process:processLabels(listener.processes),container:null,sandbox:null,local:true};
 // Preserve compatibility with older servers during a rolling binary update.
 if(sandboxes.length===1){const sandbox=sandboxes[0];return {process:processLabels([{name:sandbox.name}]),container:null,sandbox};}
 if(containers.length===1){const container=containers[0];return {process:processLabels([{name:container.name}]),container,sandbox:null};}
 return {process:processLabels(listener.processes),container:null,sandbox:null,local:true};
}
function titleCase(value=''){return value?`${value.slice(0,1).toUpperCase()}${value.slice(1)}`:'';}
function freshnessDescription(status,verifiedAt){
 if(status!=='stale')return '';
 return `\nStatus: Stale${verifiedAt?`\nLast verified: ${verifiedAt}`:''}`;
}
function containerDescription(container,hostPort,status){
 const runtime=titleCase(container.engine)||'Unknown',runtimeStatus=container.runtime?` (${String(container.runtime).toLowerCase()})`:'';
 return `Kind: Container\nContainer: ${container.name}\nMapping: ${container.container_port}/${container.protocol} → localhost:${hostPort}\nCLI: ${container.cli||'Unknown'}\nRuntime: ${runtime}${runtimeStatus}${freshnessDescription(status,container.verified_at)}`;
}
function sandboxDescription(sandbox,hostPort,status){
 return `Kind: Docker Sandbox\nSandbox: ${sandbox.name}\nMapping: ${sandbox.sandbox_port}/${sandbox.protocol} → localhost:${hostPort}\nCLI: sbx\nRuntime: Docker Sandbox${freshnessDescription(status,sandbox.verified_at)}`;
}
function localPortDescription(processes=[]){
 const plural=processes.length>1,names=processes.length?processes.map(process=>process.name||'Unknown process').join(', '):'Unknown process',pids=processes.map(process=>process.pid).filter(Boolean);
 return `Kind: Generic process\n${plural?'Processes':'Process'}: ${names}\n${pids.length>1?'PIDs':'PID'}: ${pids.length?pids.join(', '):'Unknown'}`;
}
function unknownSourceDescription(status=''){
 const details={
  ambiguous:'Multiple runtime mappings match this port.',
  degraded:'Discovery is temporarily unavailable.',
  refreshing:'Discovery is still in progress.',
  stale:'Discovery data is stale and cannot confirm this port.'
 };
 return `Kind: Unknown source\nStatus: ${details[status]||'The source could not be confirmed.'}`;
}

export function listenerPicker(ctx,port){
 const {adornmentButton,containerIndicator,el,icon,localPortIndicator,sandboxIndicator,textTooltip,warningIndicator}=ctx.ui;
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
   const {process,container,sandbox,local,unknown}=sourceLabel(listener),row=el('div','listener-option'),actions=el('div','listener-option-actions'),processNode=process.truncated?textTooltip(process.text,process.full,'listener-process').root:el('span','listener-process',process.text);row.dataset.port=String(listener.port);row.setAttribute('role','listitem');row.setAttribute('aria-label',`Port ${listener.port}: ${process.full}`);
   row.append(el('strong','listener-port',String(listener.port)),processNode);
   if(listener.listens_beyond_loopback)actions.append(warningIndicator(nonLoopbackWarning).root);
   if(sandbox)actions.append(sandboxIndicator(sandboxDescription(sandbox,listener.port,listener.source_status)).root);
   else if(container)actions.append(containerIndicator(containerDescription(container,listener.port,listener.source_status)).root);
   else if(local)actions.append(localPortIndicator(localPortDescription(listener.processes)).root);
   else if(unknown){const info=textTooltip('',unknownSourceDescription(listener.source_status),'unknown-source-indicator');info.root.prepend(icon('localPort',{className:'unknown-source-glyph'}));info.tip.className+=' unknown-source-tooltip';actions.append(info.root);}
   const use=adornmentButton(`Choose port ${listener.port}`,'arrowUp',()=>usePort(listener.port));use.className+=' listener-use-port';actions.append(use);row.append(actions);menu.append(row);
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
 const {el,numberStepperField,serviceModule}=ctx.ui;
 return serviceModule(ctx,{
  emptyState:{kind:'not-configured',message:'No local ports yet. Add a domain and an Aux port.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),port=numberStepperField('Local (Aux) port',s.aux_port||3000,{min:1,max:65535});port.input.required=true;
   const picker=listenerPicker(ctx,port);element.append(port.wrap);
   return {element,read:()=>({aux_port:Number(port.input.value)}),dispose:picker.dispose};
  }
 });
}
