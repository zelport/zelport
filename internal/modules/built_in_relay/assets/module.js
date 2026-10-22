// Header status uses persisted settings and runtime state, never form drafts.
export function headerStatus(state) {
 const d=state?.data;if(!d)return null;
 if(!d.enabled)return null;
 const s=d.status||{},label=s.state||'Stopped';
 return {status:label,tone:label==='Listening'?'ok':label==='Error'?'error':label==='Stopped'?'neutral':'pending',sections:[{rows:[
  {label:'Listeners',value:(s.listeners||[]).join('\n')||'None'},
  {label:'Clients',value:String(s.clients||0)},
  {label:'Access token',value:d.require_access_token===false?'Not required':'Required'}
 ],diagnostic:s.error||''}]};
}
export function mount(ctx) {
 const {el,field,button,adornedField,adornmentButton,dialogHeader,showDialog,statusTone,picker}=ctx.ui;
 const form=el('form','relay-settings'),enabled=field('Enable relay',false,'checkbox');
 const requireToken=field('Require access token',false,'checkbox');
 const accessMode=el('span','badge relay-open-access','Access token not required'),accessWarning=el('p','small relay-access-warning','Anyone who can reach this relay can connect and consume its capacity. Your E2E secret still protects your group’s communication. Changing this setting disconnects current clients.');
 const port=adornedField('Port',8090,'number'),bind=adornedField('Bind address','');
 const network=el('div','relay-network'),status=el('span','badge'),listeners=el('p','small muted'),error=el('p','route-error'),actions=el('div','relay-actions');
 let current,revision,dirty=false,saving=false,dialog,active=false,disposed=false,generation=0,edits=0,secretRevision;
 port.input.min='1';port.input.max='65535';port.input.required=true;bind.input.placeholder='All interfaces';
 const feedback=el('p','small muted');feedback.setAttribute('role','status');
 const reveal=adornmentButton('Reveal access token','eye',async()=>{
  if(token.input.value){clearSecret();return;}
  await readToken();
 });
 const copy=adornmentButton('Copy access token','copy',async()=>{
  const value=await readToken();if(!value)return;
  try{await navigator.clipboard.writeText(value);if(active&&!disposed)feedback.textContent='Copied to clipboard';}
  catch{if(active&&!disposed){token.input.focus();token.input.select();feedback.textContent='Copy the selected token manually.';}}
 });
 const rotate=adornmentButton('Change access token','rotate',openRotation);
 const token=adornedField('Relay access token','','password',{actions:[reveal,copy,rotate]});
 token.input.readOnly=true;token.input.autocomplete='off';token.input.spellcheck=false;
 function clearSecret(){generation++;token.input.value='';token.input.type='password';reveal.setAttribute('aria-label','Reveal access token');reveal.title='Reveal access token';feedback.textContent='';}
 async function readToken(){
  if(token.input.value)return token.input.value;
  const request=++generation,requestedRevision=ctx.ui.state().revision;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/credentials`,{method:'POST',body:JSON.stringify({revision:requestedRevision,action:'reveal'})});
   if(disposed||!active||ctx.signal.aborted||request!==generation||requestedRevision!==ctx.ui.state().revision)return;
   token.input.value=result.access_token;token.input.type='text';secretRevision=requestedRevision;
   reveal.setAttribute('aria-label','Hide access token');reveal.title='Hide access token';return result.access_token;
  }catch(e){if(!disposed&&active&&request===generation)feedback.textContent=e.message;}
 }
 function openRotation(){
  if(saving||dialog)return;
  clearSecret();
  const credentialRevision=dirty?revision:ctx.ui.state().revision,d=el('dialog','relay-token-dialog'),f=el('form'),heading=dialogHeader('Change relay access token',d);dialog=d;

  const description=el('p','muted',current.require_access_token===false?'Changing the stored token disconnects current clients. Access remains open. Clients should continue without a token. The E2E secret is unchanged.':'Changing the token disconnects all relay clients. Update each client with the new token to reconnect. The E2E secret is unchanged.');
  const mode=picker('New token',[{value:'generate',label:'Generate a new token'},{value:'replace',label:'Use an existing token'}],'',{placeholder:'Choose token action…'});
  const replacement=adornedField('New access token','','password');replacement.input.autocomplete='new-password';replacement.input.spellcheck=false;replacement.wrap.hidden=true;
  mode.onchange=()=>{replacement.wrap.hidden=mode.value!=='replace';replacement.input.required=mode.value==='replace';confirm.disabled=!mode.value;if(mode.value==='replace')replacement.input.focus();};
  const warning=el('p','route-error');warning.setAttribute('role','alert');const controls=el('div','dialog-actions'),cancel=button('Cancel',()=>d.close()),confirm=el('button','danger','Change token');confirm.type='submit';confirm.disabled=true;controls.append(cancel,confirm);
  f.append(heading,description,mode.wrap,replacement.wrap,warning,controls);d.append(f);
  f.addEventListener('submit',async e=>{
   e.preventDefault();if(saving||!['generate','replace'].includes(mode.value))return;saving=true;confirm.disabled=true;cancel.disabled=true;if(heading.children[1])heading.children[1].disabled=true;mode.disabled=true;replacement.input.disabled=true;updateButtons();warning.textContent='';
   try{
    await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/credentials`,{method:'POST',body:JSON.stringify({revision:credentialRevision,action:mode.value,access_token:mode.value==='replace'?replacement.input.value.trim():''})}),{reportError:false});
    if(revision===credentialRevision)revision=ctx.ui.state().revision;
    clearSecret();d.close();if(active&&!disposed)feedback.textContent=current.require_access_token===false?'Stored token changed. Access token is still not required.':'Token changed. Reveal or copy it to update your clients.';
   }catch(e){warning.textContent=e.message;}
   finally{saving=false;confirm.disabled=false;cancel.disabled=false;if(heading.children[1])heading.children[1].disabled=false;mode.disabled=false;replacement.input.disabled=false;updateButtons();}
  });
  d.addEventListener('cancel',e=>{if(saving)e.preventDefault();});
  showDialog(d,{signal:ctx.signal,onClose(){mode.dispose();replacement.input.value='';if(dialog===d)dialog=null;}});
 }
 requireToken.input.addEventListener('change',()=>{accessWarning.hidden=requireToken.input.checked;clearSecret();});
 network.append(bind.wrap,port.wrap,el('p','small muted','Leave the bind address blank for all interfaces, including VPN and public interfaces. Enter a local IP address to restrict the listener.'));
 const save=el('button',null,'Save settings');save.type='submit';
 const reload=button('Reload settings',async()=>{if(saving)return;dirty=false;error.textContent='';try{await ctx.refresh(true);}catch(e){error.textContent=e.message;}});
 function updateButtons(){save.disabled=saving;reload.disabled=saving;reveal.disabled=copy.disabled=saving||!current?.has_token;rotate.disabled=saving||!current?.has_token;}
 const header=el('div','relay-header'),title=el('div'),connection=el('div','relay-connection'),credentials=el('div','relay-credentials');
 title.append(el('p','muted','Host a relay for other Zelport instances. Client connections and application sharing are configured separately.'));const badges=el('div','relay-status-badges');badges.append(status,accessMode);header.append(title);
 const runtime=el('div','relay-runtime');runtime.append(badges,listeners);connection.append(enabled.wrap,runtime);credentials.append(requireToken.wrap,accessWarning,token.wrap,feedback,el('p','small muted','The stored token is used only when protection is required. It is kept when protection is disabled. Keep the E2E secret separate.'));
 actions.append(save,reload);form.append(header,connection,network,credentials,error,actions);
 form.addEventListener('input',e=>{if(e.target===token.input)return;dirty=true;edits++;updateButtons();},{signal:ctx.signal});
 form.addEventListener('submit',async e=>{
  e.preventDefault();if(saving)return;const submittedEdits=edits;saving=true;updateButtons();error.textContent='';
  try{await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/settings`,{method:'PUT',body:JSON.stringify({revision,enabled:enabled.input.checked,port:Number(port.input.value),bind_address:bind.input.value.trim(),require_access_token:requireToken.input.checked})}));dirty=edits!==submittedEdits;revision=ctx.ui.state().revision;}
  catch{/* The shell displays settings errors only on this tab. */}
  finally{saving=false;updateButtons();}
 },{signal:ctx.signal});
 ctx.panel.append(form);
 return {
  render(state){current=state.data;if(!dirty&&!saving){revision=state.revision;enabled.input.checked=current.enabled;if(port.input.value!==String(current.port))port.input.value=current.port;if(bind.input.value!==current.bind_address)bind.input.value=current.bind_address;requireToken.input.checked=current.require_access_token===true;}
   accessWarning.hidden=requireToken.input.checked;accessMode.hidden=current.require_access_token===true;
   if(token.input.value&&secretRevision!==state.revision)clearSecret();
   const s=current.status;ctx.ui.setText(status,s.state);status.className='badge status-badge';status.dataset.tone=statusTone(s.state);
   ctx.ui.setText(listeners,`${s.clients} connected clients${s.listeners.length?' · '+s.listeners.join(', '):''}${s.error?' · '+s.error:''}`);
   token.input.placeholder=current.has_token?'Token hidden':'Created when token protection is enabled';updateButtons();
  },
  activate(){active=true;},deactivate(){active=false;clearSecret();dialog?.close();},dispose(){disposed=true;active=false;clearSecret();dialog?.close();form.remove();}
 };
}
