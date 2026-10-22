const sshHostHelp='Uses the SSH configuration on this computer. Verify the same host with the system ssh command. Password, passphrase, and physical confirmation prompts are not supported.\nSource: ~/.ssh/config';

export function sshHostPicker(ctx,host){
 const {adornmentButton,el,serviceTargetIndicator}=ctx.ui;
 const menu=el('section','ssh-host-picker-menu');menu.setAttribute('role','list');menu.setAttribute('aria-label','Configured SSH hosts');host.wrap.append(menu);
 const heading=el('div','ssh-host-picker-header'),title=el('strong','ssh-host-picker-title','Configured SSH hosts'),info=serviceTargetIndicator(sshHostHelp,'SSH authentication'),refresh=adornmentButton('Refresh SSH hosts','rotate',()=>load());
 info.root.className+=' ssh-host-help';info.root.children[1].className+=' ssh-host-help-tooltip';info.root.children[0].setAttribute('aria-label','SSH authentication requirements');
 refresh.className+=' ssh-host-refresh';heading.append(title,info.root,refresh);menu.append(heading);
 let disposed=false,loading=false,request=null,hosts=[];
 function clearRows(){for(const child of [...menu.children])if(child!==heading)child.remove();}
 function render(message=''){
  clearRows();refresh.disabled=loading;refresh.setAttribute('aria-busy',String(loading));
  if(message||!hosts.length){const state=el('div','ssh-host-picker-state',message||'No SSH hosts found. Enter the SSH host manually.');state.setAttribute('role','status');menu.append(state);return;}
  for(const alias of hosts){
   const row=el('div','ssh-host-option');row.dataset.host=alias;row.setAttribute('role','listitem');
   const use=adornmentButton(`Choose SSH host ${alias}`,'arrowUp',()=>{host.input.value=alias;host.input.dispatchEvent(new Event('input',{bubbles:true}));host.input.focus();});use.className+=' ssh-host-use';
   row.append(el('strong','ssh-host-name',alias),use);menu.append(row);
  }
 }
 async function load(){
  if(disposed||loading)return;
  request?.abort();request=new AbortController();loading=true;render(hosts.length?'':'Loading SSH hosts…');
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/hosts`,{method:'GET',signal:request.signal});
   if(disposed||request.signal.aborted)return;
   hosts=Array.isArray(result.hosts)?result.hosts:[];loading=false;render();
  }catch(error){if(disposed||request.signal.aborted)return;loading=false;hosts=[];render("SSH hosts couldn't be read. Enter the SSH host manually.");}
 }
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;request?.abort();ctx.signal?.removeEventListener('abort',abort);}
 load();return {menu,dispose};
}

export function mount(ctx){
 const {el,field,serviceModule,warningIndicator}=ctx.ui;
 return serviceModule(ctx,{
  routeMenu:true,
  columns:['Service','SSH target','Info','Actions'],
  tableLabel:'Ports over SSH services',
  serviceLabel:(route,state)=>`${route.domain}:${state.port}`,
  targetLabel:route=>route.summary.target,
  targetInfo:route=>`SSH host: ${route.editor?.ssh_host||''}\nRemote loopback port: ${route.editor?.ssh_port||''}`,
  rowIndicators:route=>route.summary.detail?[warningIndicator(route.summary.detail).root]:[],
  emptyState:{kind:'not-configured',message:'No SSH ports yet. Add an SSH host and a remote port.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),host=field('SSH host',s.ssh_host||''),port=field('Remote loopback port',s.ssh_port||3000,'number');
   host.input.required=true;host.input.placeholder='office-server';port.input.min=1;port.input.max=65535;port.input.required=true;
   const picker=sshHostPicker(ctx,host);
   element.append(host.wrap,port.wrap,el('p','small muted','Uses the SSH configuration on this computer. No Aux port is allocated.'));
   return {element,read:()=>({ssh_host:host.input.value.trim(),ssh_port:Number(port.input.value)}),dispose:picker.dispose};
  }
 });
}
