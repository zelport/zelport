const sshHostHelp=path=>`Uses your SSH configuration. Password and passphrase prompts are not supported.\nSource: ${path}`;

export function sshHostPicker(ctx,host){
 const {adornmentButton,button,el,field,serviceTargetIndicator}=ctx.ui;
 const menu=el('section','ssh-host-picker-menu');menu.setAttribute('role','list');menu.setAttribute('aria-label','Configured SSH hosts');host.wrap.append(menu);
 const heading=el('div','ssh-host-picker-header'),title=el('strong','ssh-host-picker-title','Configured SSH hosts'),info=serviceTargetIndicator(sshHostHelp(ctx.ui.state?.().states?.[ctx.moduleId]?.data?.effective_config_path||'~/.ssh/config'),'SSH authentication'),refresh=adornmentButton('Refresh SSH hosts','rotate',()=>load());
 info.root.className+=' ssh-host-help';info.root.children[1].className+=' ssh-host-help-tooltip';info.root.children[0].setAttribute('aria-label','SSH authentication requirements');
 refresh.className+=' ssh-host-refresh';heading.append(title,info.root,refresh);
 const search=field('Search SSH hosts','','search');search.input.className+=' ssh-host-picker-search';search.input.placeholder='Host name';search.wrap.className+=' ssh-host-picker-filter';menu.append(heading,search.wrap);
 let disposed=false,loading=false,request=null,hosts=[],renderError='',options=[];
 function clearRows(){for(const child of [...menu.children])if(child!==heading&&child!==search.wrap)child.remove();options=[];}
 function updateSelection(){for(const {row,use,alias} of options){const selected=host.input.value.trim()===alias;row.dataset.selected=String(selected);use.textContent=selected?'Selected':'Use';use.setAttribute('aria-pressed',String(selected));}}
 function render(){
  const scroll=menu.scrollTop;clearRows();refresh.disabled=loading;refresh.setAttribute('aria-busy',String(loading));
  const query=search.input.value.trim().toLowerCase(),filtered=hosts.filter(alias=>alias.toLowerCase().includes(query));
  const message=loading&&!hosts.length?'Loading SSH hosts…':renderError||(!hosts.length?'No SSH hosts found.':!filtered.length?'No matching SSH hosts.':'');
  if(message){const state=el('div','ssh-host-picker-state',message);state.setAttribute('role','status');menu.append(state);}
  for(const alias of filtered){
   const row=el('div','ssh-host-option');row.dataset.host=alias;row.setAttribute('role','listitem');
   const use=button('Use',()=>{host.input.value=alias;host.input.dispatchEvent(new Event('input',{bubbles:true}));host.input.focus();},'secondary ssh-host-use');use.setAttribute('aria-label',`Use SSH host ${alias}`);
   row.append(el('strong','ssh-host-name',alias),use);menu.append(row);options.push({row,use,alias});
  }
  updateSelection();menu.scrollTop=scroll;
 }
 async function load(){
  if(disposed||loading)return;
  request?.abort();request=new AbortController();loading=true;render();
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/hosts`,{method:'GET',signal:request.signal});
   if(disposed||request.signal.aborted)return;
   hosts=Array.isArray(result.hosts)?result.hosts:[];if(result.config_path)info.root.children[1].textContent=sshHostHelp(result.config_path);loading=false;renderError='';render();
  }catch(error){if(disposed||request.signal.aborted)return;loading=false;renderError="SSH hosts couldn't be read. Enter the SSH host manually.";render();}
 }
 const filter=()=>render(),searchKeydown=event=>{if(event.key==='Enter')event.preventDefault();};search.input.addEventListener('input',filter);search.input.addEventListener('keydown',searchKeydown);host.input.addEventListener('input',updateSelection);
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;request?.abort();search.input.removeEventListener('input',filter);search.input.removeEventListener('keydown',searchKeydown);host.input.removeEventListener('input',updateSelection);ctx.signal?.removeEventListener('abort',abort);}
 load();return {menu,dispose};
}

export function sourceFacts(state){const d=state?.data||{};return [{label:'SSH config',value:`${d.ssh_config_path||'~/.ssh/config'}${d.config_exists===false?' (missing)':''}`,tone:d.config_exists===false?'warning':undefined}];}
export function mount(ctx){
 const {el,button,field,adornedField,adornmentButton,serviceModule,warningIndicator}=ctx.ui;
 let validation,current,form,path,baseline='',revision,dirty=false,saving=false,refreshing=false,disposed=false,webEditor=null;
 const views=ctx.ui.moduleViews(ctx,{overview:el('section','ssh-overview-view'),configuration:el('section','ssh-configuration-view'),onChange(){if(!form&&current)configure();}});
 const {overview,configuration,actions:routeToolbar}=views;
 const api=name=>`/api/modules/${ctx.moduleId}/${name}`;
 const savedPath=()=>current?.data?.ssh_config_path||'';
 const existence=el('p','small muted'),reason=el('p','small muted'),error=el('p','route-error'),feedback=el('p','small muted');
 error.setAttribute('role','alert');feedback.setAttribute('role','status');
 const save=el('button','primary','Save');save.type='submit';
 const discard=button('Discard',()=>{if(saving)return;syncSettings();validation?.clear();feedback.textContent='';updateControls();});
 const editWeb=adornmentButton('Edit file','edit',openWebEditor),refresh=adornmentButton('Refresh hosts','rotate',refreshHosts);
 function openSettings(){views.show('configuration');path?.input.focus();}
 function syncSettings(){baseline=savedPath();revision=ctx.ui.state().revision;path.input.value=baseline;dirty=false;}
 function updateControls(){
  if(!form)return;
  const busy=saving||!!webEditor,state=ctx.ui.state(),locked=!!state?.remote_admin&&!state?.remote_file_access;
  editWeb.disabled=busy||dirty||locked;
  path.input.disabled=busy||locked;save.disabled=busy||!dirty;discard.disabled=busy||!dirty;
  reason.textContent=locked?'Remote administration cannot change files on this computer.':dirty?'Save or discard changes before editing.':'';
  refresh.disabled=busy||refreshing;
  form.setAttribute('aria-busy',String(busy));
  existence.textContent=current?.data?.config_exists?'':'The file does not exist yet. Edit file creates it.';
 }
 function configure(){
  form=el('form','ssh-configuration-form');path=adornedField('SSH configuration file',savedPath(),'text',{actions:[editWeb,refresh]});path.input.placeholder='~/.ssh/config';path.input.autocomplete='off';path.input.spellcheck=false;
  const actions=el('div','ssh-settings-actions');actions.append(discard,save);
  form.append(path.wrap,el('p','small muted','Blank uses ~/.ssh/config.'),existence,error,reason,actions,feedback);
  path.input.addEventListener('input',()=>{dirty=path.input.value!==baseline;feedback.textContent='';updateControls();});
  validation=ctx.ui.formFeedback(form,error,[{input:path.input,matches:/path|file|SSH config/i}]);
  form.addEventListener('submit',saveSettings);configuration.append(form);syncSettings();updateControls();
 }
 async function saveSettings(event){
  event.preventDefault();if(saving||!dirty||disposed)return;
  saving=true;validation?.clear();feedback.textContent='';updateControls();
  try{
   await ctx.mutate(()=>ctx.api(api('settings'),{method:'PUT',body:JSON.stringify({revision,ssh_config_path:path.input.value})}),{reportError:false});
   if(disposed)return;syncSettings();feedback.textContent='Settings saved.';
  }catch(cause){
   if(disposed)return;
   if(cause.status===409){revision=ctx.ui.state().revision;error.textContent='Changed elsewhere. Your draft is kept. Save again or discard.';}
   else error.textContent=cause.message;
  }finally{saving=false;if(!disposed){updateControls();if(error.textContent)validation.show(error.textContent);}}
 }
 function openWebEditor(){
  if(disposed||editWeb.disabled||webEditor)return;
  validation?.clear();feedback.textContent='';
  webEditor=ctx.ui.fileEditor(ctx,{endpoint:api('config-file'),title:'Edit SSH configuration',trigger:editWeb,onClose(){webEditor=null;if(!disposed){updateControls();if(error.textContent)validation.show(error.textContent);}},async onSaved(){
   if(disposed)return;feedback.textContent='File saved.';
   try{await ctx.api(api('hosts'),{method:'GET'});await ctx.refresh?.(true);}catch(cause){if(!disposed)error.textContent='File saved. SSH hosts could not be refreshed.';}
  }});updateControls();
 }
 async function refreshHosts(){
  if(disposed||saving||refreshing)return;
  refreshing=true;validation?.clear();feedback.textContent='';updateControls();
  try{
   const result=await ctx.api(api('hosts'),{method:'GET'});await ctx.refresh?.(true);
   if(!disposed){const count=(result.hosts||[]).length;feedback.textContent=`${count} SSH ${count===1?'host':'hosts'} loaded.`;}
  }catch(cause){if(!disposed)error.textContent=cause.message;}
  finally{refreshing=false;if(!disposed){updateControls();if(error.textContent)validation.show(error.textContent);}}
 }
 const routes=serviceModule({...ctx,panel:overview,toolbar:routeToolbar},{
  routeMenu:true,
  columns:['Service','SSH target','Info','Actions'],
  tableLabel:'Ports over SSH services',
  serviceLabel:(route,state)=>`${route.domain}:${state.port}`,
  targetLabel:route=>route.summary.target,
  targetInfo:route=>`SSH host: ${route.editor?.ssh_host||''}\nService port on server: ${route.editor?.ssh_port||''}`,
  rowIndicators:route=>route.summary.detail?[warningIndicator(route.summary.detail).root]:[],
  emptyState:{kind:'not-configured',message:'No SSH services yet. Add an SSH host and the port where your service listens on that server.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),host=field('SSH host',s.ssh_host||''),port=field('Service port on server',s.ssh_port||3000,'number');
   host.input.required=true;host.input.placeholder='office-server';port.input.min=1;port.input.max=65535;port.input.required=true;
   const picker=sshHostPicker(ctx,host);
   element.append(host.wrap,port.wrap,el('p','small muted','The port on the server’s localhost.'));
   return {element,read:()=>({ssh_host:host.input.value.trim(),ssh_port:Number(port.input.value)}),dispose:picker.dispose};
  }
 });
 return {render(state){
  current=state;routes.render(state);if(!form)configure();
  if(form){
   if(!saving&&!dirty){if(baseline!==savedPath())syncSettings();else revision=ctx.ui.state().revision;}
   else if(!saving&&baseline===savedPath())revision=ctx.ui.state().revision;
   updateControls();
  }
 },openSettings,edit:routes.edit,remove:routes.remove,routeStatus(route){return route.summary.state==='unavailable'?{tone:'warning',label:'Unreachable',detail:route.summary.detail||'SSH connection failed.'}:{tone:'neutral',label:'Configured'};},activate(){routes.activate?.();},deactivate(){routes.deactivate?.();},dispose(){disposed=true;webEditor?.dispose();routes.dispose();views.dispose();}};
}
