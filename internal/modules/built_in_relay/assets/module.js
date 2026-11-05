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
 // Explanations live in info hints; only the open-access risk stays visible, as one warning line.
 ctx.ui.withInfo(enabled,'Lets other Zelport apps connect through this computer.');ctx.ui.withInfo(requireToken,'Changing this disconnects current clients.');
 const accessWarning=el('p','field-hint field-warning relay-access-warning');accessWarning.append(ctx.ui.icon('warning',{className:'warning-glyph'}),el('span',null,'Anyone who can reach this relay can use it.'));
 const port=adornedField('Port',8070,'number'),bind=adornedField('Bind address','');
 const network=el('div','relay-network'),error=el('p','route-error'),actions=el('div','relay-actions');
 let current,revision,dirty=false,saving=false,dialog,active=false,disposed=false,generation=0,secretRevision;
 port.input.min='1';port.input.max='65535';port.input.required=true;bind.input.placeholder='All interfaces';ctx.ui.withInfo(bind,'Blank listens on all interfaces, including VPN and public ones.');
 const feedback=el('p','small muted');feedback.setAttribute('role','status');const saveConfirmation=ctx.ui.confirmation(),copyConfirmation=ctx.ui.confirmation();
 const reveal=adornmentButton('Reveal access token','eye',async()=>{
  if(token.input.value){clearSecret();return;}
  await readToken();
 });
 const copy=adornmentButton('Copy access token','copy',async()=>{
  const value=await readToken();if(!value)return;
  try{await navigator.clipboard.writeText(value);if(active&&!disposed)copyConfirmation.show('Copied');}
  catch{if(active&&!disposed){token.input.focus();token.input.select();feedback.textContent='Copy the selected token manually.';}}
 });
 const rotate=adornmentButton('Change access token','edit',openRotation);
 const token=adornedField('Relay access token','','password',{actions:[reveal,copy,rotate]});
 token.input.readOnly=true;token.input.autocomplete='off';token.input.spellcheck=false;ctx.ui.withInfo(token,'Kept when protection is off. Never reuse the E2E secret.');
 function clearSecret(){generation++;token.input.value='';token.input.type='password';reveal.setAttribute('aria-label','Reveal access token');ctx.ui.setTooltip(reveal,'Reveal access token');feedback.textContent='';}
 async function readToken(){
  if(token.input.value)return token.input.value;
  const request=++generation,requestedRevision=ctx.ui.state().revision;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/credentials`,{method:'POST',body:JSON.stringify({revision:requestedRevision,action:'reveal'})});
   if(disposed||!active||ctx.signal.aborted||request!==generation||requestedRevision!==ctx.ui.state().revision)return;
   token.input.value=result.access_token;token.input.type='text';secretRevision=requestedRevision;
   reveal.setAttribute('aria-label','Hide access token');ctx.ui.setTooltip(reveal,'Hide access token');return result.access_token;
  }catch(e){if(!disposed&&active&&request===generation)feedback.textContent=e.message;}
 }
 function openRotation(){
  if(saving||dialog)return;
  clearSecret();
  const credentialRevision=dirty?revision:ctx.ui.state().revision,d=el('dialog','relay-token-dialog'),f=el('form'),heading=dialogHeader('Change relay access token',d);dialog=d;

  const description=el('p','muted',current.require_access_token===false?'Changing the token disconnects current clients. Access stays open.':'Changing the token disconnects all clients until they use the new token.');
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
 network.append(bind.wrap,port.wrap);
 const save=el('button','primary','Save');save.type='submit';
 function sync(){enabled.input.checked=!!current.enabled;port.input.value=String(current.port);bind.input.value=current.bind_address||'';requireToken.input.checked=current.require_access_token===true;revision=ctx.ui.state().revision;dirty=false;accessWarning.hidden=requireToken.input.checked;}
 function changed(){saveConfirmation.clear();dirty=enabled.input.checked!==!!current.enabled||Number(port.input.value)!==current.port||bind.input.value!==(current.bind_address||'')||requireToken.input.checked!==(current.require_access_token===true);updateButtons();}
 const reload=button('Discard',()=>{if(saving)return;sync();validation.clear();clearSecret();updateButtons();enabled.input.focus();});
 function updateButtons(){save.disabled=reload.disabled=saving||!dirty;reveal.disabled=copy.disabled=saving||!current?.has_token;rotate.disabled=saving||!current?.has_token;ctx.ui.busyButton(save,saving);form.setAttribute('aria-busy',String(saving));}

 const header=el('div','relay-header'),title=el('div'),connection=el('div','relay-connection'),credentials=el('div','relay-credentials');
 // Status line: dot, state, then listeners and clients as values.
 const statusDot=el('span','builtin-relay-dot'),statusLabel=el('strong',null,''),statusFacts=el('span','builtin-relay-facts');title.className='builtin-relay-status';title.append(statusDot,statusLabel,statusFacts);header.append(title);
 connection.append(enabled.wrap);token.wrap.append(copyConfirmation.root);credentials.append(requireToken.wrap,accessWarning,token.wrap,feedback);
 actions.append(saveConfirmation.root,reload,save);form.append(header,connection,network,credentials,error,actions);
 const validation=ctx.ui.formFeedback(form,error,[{input:port.input,matches:/port/i},{input:bind.input,matches:/bind|address/i}]);
 form.addEventListener('input',e=>{if(e.target===token.input)return;changed();},{signal:ctx.signal});
 form.addEventListener('submit',async e=>{
  e.preventDefault();if(saving||!dirty)return;saving=true;updateButtons();validation.clear();let failure;
  try{await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/settings`,{method:'PUT',body:JSON.stringify({revision,enabled:enabled.input.checked,port:Number(port.input.value),bind_address:bind.input.value.trim(),require_access_token:requireToken.input.checked})}),{reportError:false});revision=ctx.ui.state().revision;changed();saveConfirmation.show('Saved');}
  catch(cause){if(cause.status===409){revision=ctx.ui.state().revision;failure='Changed elsewhere. Your draft is kept. Save again or discard.';}else failure=cause.message;}
  finally{saving=false;updateButtons();if(failure)validation.show(failure);}
 },{signal:ctx.signal});
 const views=ctx.ui.moduleViews({...ctx,toolbar:null,moduleName:ctx.moduleName||'Built-in Relay'},{overviewPlacement:'before',onChange(){clearSecret();dialog?.close();}});
 const overviewError=el('p','route-error');
 overviewError.setAttribute('role','status');views.overview.className='relay-overview';views.overview.append(overviewError);views.configuration.append(form);
 return {
  openSettings(){views.show('configuration');},
  render(state){current=state.data;if(!dirty&&!saving){revision=state.revision;enabled.input.checked=current.enabled;if(port.input.value!==String(current.port))port.input.value=current.port;if(bind.input.value!==current.bind_address)bind.input.value=current.bind_address;requireToken.input.checked=current.require_access_token===true;}
   accessWarning.hidden=requireToken.input.checked;
   if(token.input.value&&secretRevision!==state.revision)clearSecret();
   const s=current.status;
   ctx.ui.setText(statusLabel,s.state);statusDot.dataset.tone=statusTone(s.state);ctx.ui.setText(statusFacts,[...(s.listeners||[]),`${s.clients} ${s.clients===1?'client':'clients'}`].join(' · '));ctx.ui.setText(overviewError,s.error||'');views.overview.hidden=!s.error;
   token.input.placeholder=current.has_token?'Token hidden':'Created when token protection is enabled';updateButtons();
  },
  activate(){active=true;},deactivate(){active=false;clearSecret();dialog?.close();},dispose(){disposed=true;active=false;clearSecret();dialog?.close();views.dispose();}
 };
}
