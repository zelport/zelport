const discoveryFailure="Listening ports couldn't be detected. Enter the Aux port manually.";
const nonLoopbackWarning='The port listens on non-loopback interfaces too.';

function processLabel(processes=[]){
 if(!processes.length)return 'Unknown process';
 return processes.map(process=>`${process.name||'Unknown process'}${process.pid?` (${process.pid})`:''}`).join(', ');
}

export function listenerPicker(ctx,port){
 const {adornmentButton,disclosureMenu,el,warningIndicator}=ctx.ui;
 const actions=el('div','input-adornment-actions'),menu=el('div','mapping-menu listener-picker-menu');
 const trigger=adornmentButton('Choose a listening port','rotate',()=>{if(!loaded)return load();});
 trigger.className+=' listener-picker-trigger';trigger.setAttribute('aria-haspopup','listbox');
 menu.setAttribute('role','listbox');menu.setAttribute('aria-label','Listening ports');
 actions.append(trigger);port.control.append(actions);port.wrap.append(menu);

 let loaded=false,loading=false,disposed=false,generation=0,request=null,pendingFocus=0;
 const heading=el('div','listener-picker-header'),title=el('strong',null,'Listening ports');heading.disabled=true;
 const refresh=adornmentButton('Refresh listening ports','rotate',()=>load(true));refresh.className+=' listener-picker-refresh';
 heading.append(title,refresh);menu.append(heading);
 const disclosure=disclosureMenu(port.wrap,trigger,menu,{selected:node=>node.dataset.port===String(port.input.value)});

 function clearRows(){for(const child of [...menu.children])if(child!==heading)child.remove();}
 function state(message,{spinner=false,warning=false}={}){
  const row=el('div',`listener-picker-state${warning?' listener-picker-state-warning':''}`);row.disabled=true;row.setAttribute('role','status');
  if(spinner){row.setAttribute('aria-label','Loading listening ports');const indicator=el('span','boot-spinner');indicator.setAttribute('aria-hidden','true');row.append(indicator);}
  else row.append(el('span','listener-picker-message',message));
  menu.append(row);
 }
 function select(row){
  port.input.value=row.dataset.port;port.input.dispatchEvent(new Event('input',{bubbles:true}));
  for(const option of menu.children)if(option.dataset.port)option.setAttribute('aria-selected',String(option===row));
  disclosure.select();trigger.focus();
 }
 function render(listeners,error=''){
  const scroll=menu.scrollTop;clearRows();refresh.disabled=loading;refresh.setAttribute('aria-busy',String(loading));
  if(loading)state('',{spinner:true});
  else if(error)state(discoveryFailure,{warning:true});
  else if(!listeners.length)state('No listening ports found.');
  else for(const listener of listeners){
   const row=el('div','listener-option');row.dataset.port=String(listener.port);row.tabIndex=-1;row.setAttribute('role','option');row.setAttribute('aria-selected',String(row.dataset.port===String(port.input.value)));
   row.append(el('strong','listener-port',String(listener.port)),el('span','listener-process',processLabel(listener.processes)));
   if(listener.listens_beyond_loopback){const warning=warningIndicator(nonLoopbackWarning);warning.root.addEventListener('click',event=>event.stopPropagation());row.append(warning.root);}
   row.addEventListener('click',()=>select(row));row.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();select(row);}});menu.append(row);
  }
  menu.scrollTop=scroll;
  if(!loading&&!error&&pendingFocus){const rows=[...menu.children].filter(child=>child.dataset.port);(pendingFocus<0?rows.at(-1):rows[0])?.focus();pendingFocus=0;}
 }
 async function load(force=false){
  if(disposed||loading&&!force)return;
  request?.abort();request=new AbortController();const current=++generation;loading=true;render([]);
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/listeners`,{method:'GET',signal:request.signal});
   if(disposed||current!==generation)return;loaded=true;loading=false;render(Array.isArray(result.listeners)?result.listeners:[]);
  }catch(error){
   if(disposed||current!==generation||request.signal.aborted)return;
   loaded=false;loading=false;render([],discoveryFailure);
  }
 }
 trigger.addEventListener('keydown',event=>{if((event.key==='ArrowDown'||event.key==='ArrowUp')&&!loaded){pendingFocus=event.key==='ArrowUp'?-1:1;load();}});
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;generation++;request?.abort();disclosure.dispose();ctx.signal?.removeEventListener('abort',abort);}
 return {trigger,menu,dispose};
}

export function mount(ctx){
 const {el,field,serviceModule}=ctx.ui;
 return serviceModule(ctx,{
  empty:'No local ports yet. Add a domain and an Aux port.',
  fields(s){
   const element=el('div'),port=field('Aux port',s.aux_port||3000,'number');port.input.min=1;port.input.max=65535;port.input.required=true;
   const picker=listenerPicker(ctx,port);element.append(port.wrap);
   return {element,read:()=>({aux_port:Number(port.input.value)}),dispose:picker.dispose};
  }
 });
}
