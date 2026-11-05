const sshHostHelp=path=>`Uses your SSH configuration. Password and passphrase prompts are not supported.\nSource: ${path}`;

// The SSH host field is also the filter for the configured hosts listed below it.
export function sshHostPicker(ctx,host,{container=host.wrap}={}){
 const {adornmentButton,choiceList,choiceRow,choiceState,el,serviceTargetIndicator}=ctx.ui;
 const menu=choiceList('Configured SSH hosts');menu.className+=' ssh-host-picker-menu';container.append(menu);
 const info=serviceTargetIndicator(sshHostHelp(ctx.ui.state?.().states?.[ctx.moduleId]?.data?.effective_config_path||'~/.ssh/config'),'SSH authentication'),refresh=adornmentButton('Refresh SSH hosts','rotate',()=>load());
 info.root.className+=' ssh-host-help';info.root.children[1].className+=' ssh-host-help-tooltip';info.root.children[0].setAttribute('aria-label','SSH authentication requirements');
 refresh.className+=' ssh-host-refresh';
 const tools=el('div','input-adornment-actions');tools.append(refresh);(host.control||host.wrap).append(tools);
 // The SSH help belongs to the field label, not inside the input.
 const [caption,...rest]=[...(host.wrap.children||[])];if(caption?.tagName==='LABEL'&&host.wrap.replaceChildren){const heading=el('div','field-label-row');heading.append(caption,info.root);host.wrap.replaceChildren(heading,...rest);}else (host.control||host.wrap).append(info.root);
 host.input.setAttribute('aria-controls',menu.id);
 let disposed=false,loading=false,request=null,hosts=[],renderError='',options=[],query='',filling=false;
 function clearRows(){menu.replaceChildren();options=[];}
 function updateSelection(){for(const option of options)option.select(host.input.value.trim()===option.alias);}
 function render(){
  const scroll=menu.scrollTop;clearRows();refresh.disabled=loading;refresh.setAttribute('aria-busy',String(loading));
  const filtered=hosts.filter(alias=>alias.toLowerCase().includes(query));
  const message=loading&&!hosts.length?'Loading SSH hosts…':renderError||(!hosts.length?'No SSH hosts found.':!filtered.length?'No configured host matches.':'');
  if(message){const state=choiceState(message,{warning:!!renderError&&message===renderError});state.className+=' ssh-host-picker-state';menu.append(state);}
  for(const alias of filtered){
   const choice=choiceRow({primary:alias,label:`Use SSH host ${alias}`,onSelect:()=>{query='';filling=true;host.input.value=alias;host.input.dispatchEvent(new Event('input',{bubbles:true}));filling=false;render();host.input.focus();}});
   choice.row.className+=' ssh-host-option';choice.row.dataset.host=alias;choice.row.setAttribute('role','listitem');choice.main.className+=' ssh-host-use';choice.main.children[0].className+=' ssh-host-name';
   menu.append(choice.row);options.push({...choice,alias});
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
 const typed=()=>{if(filling)return;query=host.input.value.trim().toLowerCase();render();};host.input.addEventListener('input',typed);
 const abort=()=>dispose();ctx.signal?.addEventListener('abort',abort,{once:true});
 function dispose(){if(disposed)return;disposed=true;request?.abort();host.input.removeEventListener('input',typed);ctx.signal?.removeEventListener('abort',abort);}
 load();return {menu,dispose};
}

export function mount(ctx){
 const {el,button,field,adornedField,adornmentButton,serviceModule,warningIndicator}=ctx.ui;
 let validation,current,form,path,baseline='',revision,dirty=false,saving=false,refreshing=false,disposed=false,webEditor=null;
 const views=ctx.ui.moduleViews(ctx,{overview:el('section','ssh-overview-view'),configuration:el('section','ssh-configuration-view'),onChange(){if(!form&&current)configure();}});
 const {overview,configuration,actions:routeToolbar}=views;
 const api=name=>`/api/modules/${ctx.moduleId}/${name}`;
 const savedPath=()=>current?.data?.ssh_config_path||'';
 const existence=el('p','small muted'),reason=el('p','small muted'),error=el('p','route-error'),feedback=ctx.ui.confirmation(),fileFeedback=ctx.ui.confirmation();
 error.setAttribute('role','alert');
 const save=el('button','primary','Save');save.type='submit';
 const discard=button('Discard',()=>{if(saving)return;syncSettings();validation?.clear();feedback.clear();fileFeedback.clear();updateControls();});
 const hostsCount=el('span','ssh-hosts-count'),hostsList=el('div','ssh-hosts');hostsList.setAttribute('role','list');
 function renderHosts(hosts=[]){ctx.ui.setText(hostsCount,String(hosts.length));hostsList.replaceChildren(...hosts.map(host=>{const chip=el('span','ssh-host-chip',host);chip.setAttribute('role','listitem');return chip;}));}
 async function loadHosts(){try{const result=await ctx.api(api('hosts'),{method:'GET'});if(!disposed)renderHosts(result.hosts||[]);}catch{if(!disposed)renderHosts([]);}}
 const editWeb=adornmentButton('Edit file','edit',openWebEditor),refresh=adornmentButton('Refresh hosts','rotate',refreshHosts);
 function openSettings(){views.show('configuration');path?.input.focus();}
 function syncSettings(){baseline=savedPath();revision=ctx.ui.state().revision;path.input.value=baseline;dirty=false;}
 function updateControls(){
  if(!form)return;
  const busy=saving||!!webEditor,state=ctx.ui.state(),locked=!!state?.remote_admin&&!state?.remote_file_access;
  editWeb.disabled=busy||dirty||locked;
  path.input.disabled=busy||locked;save.disabled=busy||!dirty;ctx.ui.busyButton(save,saving);discard.disabled=busy||!dirty;
  reason.textContent=locked?'Remote administration cannot change files on this computer.':dirty?'Save or discard changes before editing.':'';
  refresh.disabled=busy||refreshing;
  form.setAttribute('aria-busy',String(busy));
  existence.textContent=current?.data?.config_exists?'':'The file does not exist yet. Edit file creates it.';
 }
 function configure(){
  form=el('form','ssh-configuration-form');path=adornedField('SSH config file',savedPath(),'text',{actions:[editWeb,refresh]});ctx.ui.withInfo(path,'Blank uses ~/.ssh/config. Zelport connects with your SSH keys and agent, without password prompts.');path.input.placeholder='~/.ssh/config';path.input.autocomplete='off';path.input.spellcheck=false;
  const actions=el('div','ssh-settings-actions');actions.append(feedback.attach(save).root,discard,save);path.wrap.append(fileFeedback.root);
  // Hosts from the SSH config: label + count, then the aliases as chips.
  const hostsHead=el('div','field-label-row ssh-hosts-head');hostsHead.append(el('span','ssh-hosts-label','Hosts'),hostsCount);
  form.append(path.wrap,existence,hostsHead,hostsList,error,reason,actions);loadHosts();
  path.input.addEventListener('input',()=>{dirty=path.input.value!==baseline;feedback.clear();fileFeedback.clear();updateControls();});
  validation=ctx.ui.formFeedback(form,error,[{input:path.input,matches:/path|file|SSH config/i}]);
  form.addEventListener('submit',saveSettings);configuration.append(form);syncSettings();updateControls();
 }
 async function saveSettings(event){
  event.preventDefault();if(saving||!dirty||disposed)return;
  saving=true;validation?.clear();feedback.clear();fileFeedback.clear();updateControls();
  try{
   await ctx.mutate(()=>ctx.api(api('settings'),{method:'PUT',body:JSON.stringify({revision,ssh_config_path:path.input.value})}),{reportError:false});
   if(disposed)return;syncSettings();feedback.show('Saved');
  }catch(cause){
   if(disposed)return;
   if(cause.status===409){revision=ctx.ui.state().revision;error.textContent='Changed elsewhere. Your draft is kept. Save again or discard.';}
   else error.textContent=cause.message;
  }finally{saving=false;if(!disposed){updateControls();if(error.textContent)validation.show(error.textContent);}}
 }
 function openWebEditor(){
  if(disposed||editWeb.disabled||webEditor)return;
  validation?.clear();feedback.clear();fileFeedback.clear();
  webEditor=ctx.ui.fileEditor(ctx,{endpoint:api('config-file'),title:'Edit SSH configuration',trigger:editWeb,onClose(){webEditor=null;if(!disposed){updateControls();if(error.textContent)validation.show(error.textContent);}},async onSaved(){
   if(disposed)return;fileFeedback.show('File saved');
   try{const result=await ctx.api(api('hosts'),{method:'GET'});renderHosts(result.hosts||[]);await ctx.refresh?.(true);}catch(cause){if(!disposed)error.textContent='File saved. SSH hosts could not be refreshed.';}
  }});updateControls();
 }
 async function refreshHosts(){
  if(disposed||saving||refreshing)return;
  refreshing=true;validation?.clear();feedback.clear();fileFeedback.clear();updateControls();
  try{
   const result=await ctx.api(api('hosts'),{method:'GET'});await ctx.refresh?.(true);
   if(!disposed)renderHosts(result.hosts||[]);
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
   const element=el('div','service-fields-fill'),host=field('SSH host',s.ssh_host||''),port=field('Server port',s.ssh_port||3000,'number');
   host.input.required=true;host.input.autocomplete='off';host.input.spellcheck=false;host.input.placeholder='Host or search';host.control.prepend(ctx.ui.icon('search',{className:'input-adornment-icon'}));
   port.input.min=1;port.input.max=65535;port.input.required=true;port.wrap.className+=' field-port';
   element.append(ctx.ui.fieldRow(host.wrap,port.wrap));const picker=sshHostPicker(ctx,host,{container:element});
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
 },openSettings,edit:routes.edit,remove:routes.remove,routeStatus(route){
  // The connection pool state: idle until the first request, then connecting / connected, or error.
  const connection=ctx.ui.state()?.states?.[ctx.moduleId]?.data?.connections?.[route.id];
  if(route.summary.state==='unavailable'||connection?.state==='error')return {tone:'error',label:'Unreachable',detail:route.summary.detail||'SSH connection failed.'};
  if(connection?.state==='connected')return {tone:'ok',label:'Connected'};
  if(connection?.state==='connecting')return {tone:'neutral',label:'Connecting'};
  return {tone:'neutral',label:'Idle',detail:'Connects on the first request.'};
 },activate(){routes.activate?.();},deactivate(){routes.deactivate?.();},dispose(){disposed=true;webEditor?.dispose();routes.dispose();views.dispose();}};
}
