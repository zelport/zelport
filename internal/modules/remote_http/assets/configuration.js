export function mountConfiguration(ctx,panel){
 const {el,field,button,adornedField,adornmentButton}=ctx.ui;
 let validation,current={},form,ca,proxy,username,password,clear,revision,baseline='',dirty=false,saving=false,reloading=false,disposed=false,webEditor=null;
 const error=el('p','route-error'),runtime=el('p','route-error'),feedback=ctx.ui.confirmation(),fileFeedback=ctx.ui.confirmation(),editorReason=el('p','small muted');
 error.setAttribute('role','alert');runtime.setAttribute('role','status');
 const save=el('button','primary','Save');save.type='submit';
 const discard=button('Discard',()=>{if(saving||reloading)return;sync();validation?.clear();feedback.clear();fileFeedback.clear();update();});
 const editWeb=adornmentButton('Edit file','edit',openWebEditor),reloadCA=adornmentButton('Reload certificates','rotate',reloadCertificates);
 const signature=()=>JSON.stringify([current.ca_cert_path||'',current.proxy_url||'',current.proxy_username||'',!!current.has_proxy_password]);
 function sync(){
  ca.input.value=current.ca_cert_path||'';proxy.input.value=current.proxy_url||'';username.input.value=current.proxy_username||'';
  password.input.value='';clear.input.checked=false;baseline=signature();revision=ctx.ui.state().revision;dirty=false;
 }
 function update(){
  if(!form)return;
  const direct=!proxy.input.value.trim(),busy=saving||reloading||!!webEditor;
  for(const control of [ca,proxy,username,password,clear])control.input.disabled=busy;
  username.input.disabled=busy||direct;password.input.disabled=busy||direct||clear.input.checked;clear.input.disabled=busy||direct;
  const sameProxy=proxy.input.value.trim()===(current.proxy_url||'')&&username.input.value===(current.proxy_username||'');
  password.input.placeholder=current.has_proxy_password&&sameProxy?'Stored password':'';
  clear.wrap.hidden=!current.has_proxy_password;
  ca.input.placeholder=current.default_ca_cert_display_path||current.default_ca_cert_path||'~/.config/zelport/additional-ca.pem';
  const state=ctx.ui.state(),locked=!!state?.remote_admin&&!state?.remote_file_access;ca.input.disabled=busy||locked;
  editWeb.disabled=busy||dirty||locked;
  editorReason.textContent=locked?'Remote administration cannot change files on this computer.':dirty?'Save or discard changes before editing.':'';reloadCA.disabled=busy;
  save.disabled=discard.disabled=busy||!dirty;ctx.ui.busyButton(save,saving);form.setAttribute('aria-busy',String(busy));runtime.textContent=current.configuration_error||'';
 }
 function open(focus=true){
  if(disposed)return;
  if(!form){
   form=el('form','remote-configuration-form');ca=adornedField('Additional CA certificate file','','text',{actions:[editWeb,reloadCA]});proxy=field('Proxy URL','','url');username=field('Proxy username');password=field('Proxy password','','password');clear=field('Clear stored password',false,'checkbox');
   proxy.input.placeholder='http://proxy.example.com:8080';
   ca.input.spellcheck=proxy.input.spellcheck=username.input.spellcheck=false;ca.input.autocomplete=proxy.input.autocomplete=username.input.autocomplete='off';password.input.autocomplete='new-password';
   const auth=el('details','remote-proxy-auth'),actions=el('div','remote-settings-actions');auth.append(el('summary',null,'Authentication'),username.wrap,password.wrap,clear.wrap);actions.append(feedback.root,discard,save);ca.wrap.append(fileFeedback.root);
   ctx.ui.withInfo(ca,'Trusted in addition to system certificates.');ctx.ui.withInfo(proxy,'HTTP(S) or SOCKS5. Blank connects directly.');
   form.append(ca.wrap,editorReason,runtime,proxy.wrap,auth,error,actions);
   form.addEventListener('input',()=>{dirty=ca.input.value!==(current.ca_cert_path||'')||proxy.input.value!==(current.proxy_url||'')||username.input.value!==(current.proxy_username||'')||!!password.input.value||clear.input.checked;feedback.clear();fileFeedback.clear();update();});
   validation=ctx.ui.formFeedback(form,error,[{input:ca.input,matches:/certificate|PEM|CA /i},{input:proxy.input,matches:/proxy|URL|scheme/i}]);
   form.addEventListener('submit',submit);panel.append(form);sync();update();
  }
  if(focus)ca.input.focus();
 }
 async function submit(event){
  event.preventDefault();if(disposed||saving||reloading||!dirty)return;
  saving=true;validation?.clear();feedback.clear();fileFeedback.clear();update();
  try{
   await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/settings`,{method:'PUT',body:JSON.stringify({revision,ca_cert_path:ca.input.value,proxy_url:proxy.input.value.trim(),proxy_username:username.input.value,proxy_password:clear.input.checked?'':password.input.value,clear_proxy_password:clear.input.checked})}),{reportError:false});
   if(disposed)return;sync();feedback.show('Saved');
  }catch(cause){
   if(disposed)return;
   if(cause.status===409){revision=ctx.ui.state().revision;error.textContent='Changed elsewhere. Your draft is kept. Save again or discard.';}
   else error.textContent=cause.message;
  }finally{saving=false;if(!disposed){update();if(error.textContent)validation.show(error.textContent);}}
 }
 function openWebEditor(){
  if(disposed||editWeb.disabled||webEditor)return;
  validation?.clear();feedback.clear();fileFeedback.clear();
  webEditor=ctx.ui.fileEditor(ctx,{endpoint:`/api/modules/${ctx.moduleId}/ca-file`,title:'Edit CA certificates',trigger:editWeb,onClose(){webEditor=null;if(!disposed){update();if(error.textContent)validation.show(error.textContent);}},async onSaved(){
   if(disposed)return;fileFeedback.show('File saved');
   try{await ctx.refresh(true);}catch(cause){if(!disposed)error.textContent='File saved. Certificate status could not be refreshed.';}
  }});update();
 }
 async function reloadCertificates(){
  if(disposed||saving||reloading)return;
  reloading=true;validation?.clear();feedback.clear();fileFeedback.clear();update();
  try{await ctx.refresh(true);if(!disposed&&!current.configuration_error)fileFeedback.show('Reloaded');}
  catch(cause){if(!disposed)error.textContent=cause.message;}
  finally{reloading=false;if(!disposed){update();if(error.textContent)validation.show(error.textContent);}}
 }
 return {open,render(state){current=state.data||{};if(form){if(!dirty&&!saving){if(signature()!==baseline)sync();else revision=ctx.ui.state().revision;}update();}},dispose(){disposed=true;webEditor?.dispose();if(password)password.input.value='';form?.remove();}};
}
