export function headerStatus(state) {
 const d=state?.data;if(!d)return null;
 const s=d.status||{};let endpoint='Not configured';
 try{endpoint=new URL(d.relay_url).host;}catch{}
 const rows=[{label:'Relay',value:endpoint},{label:'Sharing',value:d.publish?'Enabled':'Disabled'},{label:'Imports',value:String(state.routes?.length||0)}];
 if(!d.enabled)return {status:'Disconnected',tone:'neutral',rows};
 if(s.error)rows.push({label:'Connection',value:'Retrying. Open settings for error details.'});
 return {status:s.connected?'Connected':s.error?'Error · retrying':'Connecting',tone:s.connected?'ok':s.error?'error':'pending',rows};
}
export function mount(ctx) {
 const {el,button,field,serviceModule,renderChanged,adornedField,adornmentButton,dialogHeader,showDialog,formActions,statusBadge,refreshButton}=ctx.ui;
 let current,dialog,secretInput,secretReveal,secretFeedback,secretRevision,active=false,generation=0,refreshing=false;
 const routePanel=el('section'),setup=el('section','cloud-setup'),catalog=el('section','cloud-catalog'),administration=el('section','cloud-administration');
 ctx.panel.append(setup,el('h2',null,'Imported services'),routePanel,administration,catalog);
 const api=path=>`/api/modules/${ctx.moduleId}/${path}`;
 const routes=serviceModule({...ctx,panel:routePanel},{add:false,empty:'No imported services yet. Available shared services can be imported below.',fields(settings){
  const element=el('div'),origin=field('Rewrite Origin',settings.rewrite_origin,'checkbox');
  element.append(el('p','small muted',`Provider ${settings.device_id} · Service ${settings.service_id}`),origin.wrap);
  return {element,read:()=>({...settings,rewrite_origin:origin.input.checked})};
 }});
 function configure(){
  dialog?.close();const d=el('dialog','cloud-settings-dialog'),form=el('form');dialog=d;
  const data=current.data,revision=ctx.ui.state().revision;
  const enabled=field('Connect to relay',data.enabled,'checkbox'),publish=field('Share all own services',data.publish,'checkbox'),url=field('Relay URL',data.relay_url),access=field('Relay access token','','password'),group=field('E2E secret','','password'),useAccess=field('Use access token',data.has_access_token,'checkbox');
  url.input.placeholder='wss://relay.example.com';access.input.autocomplete='off';group.input.autocomplete='off';
  const accessHint=el('p','small muted');
  const updateAccess=()=>{access.input.disabled=!useAccess.input.checked;access.input.required=useAccess.input.checked&&!data.has_access_token;accessHint.textContent=useAccess.input.checked?'Leave the access-token field blank to keep the stored token.':'Connect without an access token. The relay must explicitly allow this; its identity is not authenticated by the admission channel. Saving clears any stored access token.';};
  useAccess.input.addEventListener('change',updateAccess);updateAccess();
  const advanced=el('details','dialog-advanced');
  advanced.append(el('summary',null,'Access and encryption'),useAccess.wrap,access.wrap,accessHint,group.wrap,el('p','dialog-help','Leave blank to keep the existing E2E secret, or generate one on first setup. Use the same secret on your devices. Never share it with the relay.'));
  form.append(dialogHeader('Relay settings',d),url.wrap,enabled.wrap,publish.wrap,el('p','dialog-help','Sharing includes current and future own services, excluding relay imports. Connected devices in the same E2E group always have full administration access.'),advanced);
  const {error,actions,save}=formActions(d,{submit:'Save settings',draft:true,form});form.append(error,actions);
  form.addEventListener('submit',async e=>{e.preventDefault();save.disabled=true;error.textContent='';try{await ctx.mutate(()=>ctx.api(api('settings'),{method:'PUT',body:JSON.stringify({revision,relay_url:url.input.value.trim(),access_token:useAccess.input.checked?access.input.value.trim():'',clear_access_token:!useAccess.input.checked,group_token:group.input.value.trim(),enabled:enabled.input.checked,publish:publish.input.checked})}),{reportError:false});d.close();}catch(e){error.textContent=e.message;}finally{save.disabled=false;}});
  d.append(form);showDialog(d,{signal:ctx.signal,initialFocus:url.input,onClose(){access.input.value='';group.input.value='';if(dialog===d)dialog=null;}});
 }
 function clearSecret(){generation++;if(secretInput){secretInput.value='';secretInput.type='password';}if(secretReveal){const label=current?.data.has_identity?'Reveal E2E secret':'Create E2E secret';secretReveal.setAttribute('aria-label',label);secretReveal.title=label;}if(secretFeedback)ctx.ui.setText(secretFeedback,'');}
 async function credentials(copy=false){
  const request=++generation,revision=ctx.ui.state().revision;
  const action=current.data.has_identity?'reveal':'generate';
  const result=await ctx.mutate(()=>ctx.api(api('credentials'),{method:'POST',body:JSON.stringify({revision,action})}));
  if(!active||ctx.signal?.aborted||request!==generation||!secretInput||(action==='reveal'&&revision!==ctx.ui.state().revision))return;
  secretInput.value=result.group_token;secretInput.type='text';secretRevision=ctx.ui.state().revision;secretReveal.setAttribute('aria-label','Hide E2E secret');secretReveal.title='Hide E2E secret';
  if(copy){try{await navigator.clipboard.writeText(result.group_token);ctx.ui.setText(secretFeedback,'Copied to clipboard');}catch{secretInput.focus();secretInput.select();ctx.ui.setText(secretFeedback,'Copy the selected secret manually.');}}
 }
 async function refreshCatalog(){if(refreshing)return;refreshing=true;refresh.disabled=true;try{await ctx.mutate(()=>ctx.api(api('refresh'),{method:'POST',body:'{}'}));}finally{refreshing=false;refresh.disabled=false;}}
 const refresh=refreshButton('Refresh catalog',refreshCatalog);ctx.toolbar?.append(refresh);
 function render(state){current=state;if(secretInput?.value&&secretRevision!==ctx.ui.state().revision)clearSecret();routes.render(state);const d=state.data||{},status=d.status||{records:[]};renderChanged(setup,{identity:d.has_identity,enabled:d.enabled,publish:d.publish,connected:status.connected,error:status.error},()=>{
  if(secretInput?.value)clearSecret();setup.replaceChildren();const actions=el('div','cloud-actions'),header=el('div','cloud-section-header');
  const label=status.connected?'Connected':d.enabled?(status.error?'Error':'Connecting'):'Disconnected';
  header.append(el('h2',null,'Relay connection'),statusBadge(label));
  actions.append(button('Settings',configure));
  const reveal=adornmentButton(d.has_identity?'Reveal E2E secret':'Create E2E secret','eye',()=>{if(secretInput.value){clearSecret();return;}return credentials();}),copy=adornmentButton('Copy E2E secret','copy',()=>credentials(true));copy.disabled=!d.has_identity;secretReveal=reveal;secretFeedback=el('p','small muted');secretFeedback.setAttribute('role','status');
  const secret=adornedField('E2E secret','','password',{actions:[reveal,copy]});secretInput=secret.input;secretInput.readOnly=true;secretInput.autocomplete='off';secretInput.spellcheck=false;secretInput.placeholder=d.has_identity?'Secret hidden':'Create a secret to connect your devices';
  setup.append(header,el('p','small muted',d.publish?'Sharing own application services':'Application sharing disabled'),secret.wrap,secretFeedback,actions);if(status.error)setup.append(el('p','route-error',status.error));
 });
  renderChanged(administration,{remote:!!ctx.ui.state().remote_admin,peers:d.administration_peers},()=>{administration.replaceChildren();
  if(!ctx.ui.state().remote_admin){
   administration.append(el('h2',null,'Zelport administration'));
   const peers=d.administration_peers||[];
   if(!peers.length)administration.append(el('p','muted','No other Zelport instances discovered yet.'));
   for(const peer of peers){
    const card=el('div','cloud-admin-peer');card.append(el('strong',null,peer.hostname),statusBadge(peer.available?'Available':'Unavailable'));
    for(const address of [peer.hostname_url,peer.uuid_url].filter(Boolean)){const link=el(peer.available?'a':'span','cloud-admin-address',address.replace('http://',''));if(peer.available){link.href=address;link.target='_blank';link.rel='noopener';}card.append(link);}
    if(peer.conflict)card.append(el('p','route-error small','Hostname conflict. Use the UUID address.'));
    administration.append(card);
   }
  }
  });
  const providers=(status.records||[]).filter(r=>r.device_id!==d.device_id&&r.publish&&r.services.some(s=>s.kind==='application'));
  renderChanged(catalog,{providers:providers.map(p=>({group:p.group,device_id:p.device_id,hostname:p.hostname,port:p.port,public_key:p.public_key,services:p.services,expired:p.expires_at*1000<=Date.now()})),connected:status.connected,imports:(state.routes||[]).map(r=>r.editor)},()=>{catalog.replaceChildren(el('h2',null,'Shared services'));
  if(!providers.length)catalog.append(el('p','muted','No shared services available.'));
  for(const provider of providers){
   const section=el('section','cloud-provider'),heading=el('h3',null,`${provider.hostname} · ${provider.device_id.slice(0,8)}`);
   heading.id=`cloud-provider-${provider.device_id}`;
   const unavailable=!status.connected?'Connect to the relay to import services.':provider.expires_at*1000<=Date.now()?'Provider unavailable. Imports resume when it reconnects.':'';
   const wrap=el('div','cloud-services-wrap'),table=el('table','cloud-services-table data-table'),head=el('thead'),labels=el('tr'),body=el('tbody');
   table.setAttribute('aria-labelledby',heading.id);
   for(const title of ['Domain','Localhost port','Action']){const cell=el('th',null,title);cell.scope='col';labels.append(cell);}
   head.append(labels);table.append(head,body);
   for(const service of provider.services.filter(s=>s.kind==='application')){
    const row=el('tr'),domain=el('td',null,service.domain),port=el('td',null,String(provider.port)),action=el('td');
    const imported=(state.routes||[]).some(r=>r.editor.group===provider.group&&r.editor.device_id===provider.device_id&&r.editor.service_id===service.id);
    const add=button(imported?'Add another domain':'Import',()=>routes.edit({id:'',domain:service.domain,editor:{group:provider.group,device_id:provider.device_id,service_id:service.id,provider_key:provider.public_key,rewrite_origin:false}}));
    add.disabled=!!unavailable;if(unavailable){add.title=unavailable;add.setAttribute('aria-describedby',`${heading.id}-availability`);}
    domain.dataset.label='Domain';port.dataset.label='Localhost port';action.dataset.label='Actions';action.append(add);row.append(domain,port,action);body.append(row);
   }
   wrap.append(table);section.append(heading);if(unavailable){const reason=el('p','small muted',unavailable);reason.id=`${heading.id}-availability`;section.append(reason);}section.append(wrap);catalog.append(section);
  }
  });
 }
 return {render,edit:routes.edit,activate(){active=true;},deactivate(){active=false;clearSecret();dialog?.close();},dispose(){active=false;clearSecret();refresh.remove();routes.dispose();dialog?.close();dialog?.remove();}};
}
