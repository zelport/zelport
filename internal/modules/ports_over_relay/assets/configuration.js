// Stored credentials are never treated as replacement drafts.
export function mountConfiguration(ctx) {
 const {el,field,button,adornedField,adornmentButton,setTooltip}=ctx.ui;
 const form=el('form','relay-configuration-form'),url=field('Relay URL'),access=field('Relay access token','','password');
 const modeWrap=el('div','form-field'),modeLabel=el('label',null,'Relay access'),mode=el('select');
 mode.id=`${access.input.id}-mode`;modeLabel.htmlFor=mode.id;
 for(const [value,label] of [['none','No token'],['token','Access token']]){const option=el('option',null,label);option.value=value;mode.append(option);}
 modeWrap.append(modeLabel,mode);
 let saved={},revision=0,dirty=false,saving=false,editing=false,active=false,disposed=false,generation=0,revealedRevision;
 const feedback=el('p','small muted relay-secret-feedback');feedback.setAttribute('role','status');const saveConfirmation=ctx.ui.confirmation(),copyConfirmation=ctx.ui.confirmation();
 const reveal=adornmentButton('Reveal E2E secret','eye',()=>revealSecret());
 const copy=adornmentButton('Copy E2E secret','copy',()=>copySecret());
 const change=adornmentButton('Change E2E secret','edit',()=>{generation++;editing=true;group.input.type='password';setSecret('');feedback.textContent='';changed();secretInput().focus();});
 const generate=adornmentButton('Generate E2E secret','shuffle',()=>{
  const alphabet='abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';let secret='';
  // Reject the uneven tail before mapping bytes onto the 62-character alphabet.
  while(secret.length<40){for(const byte of crypto.getRandomValues(new Uint8Array(64))){if(byte>=248)continue;secret+=alphabet[byte%62];if(secret.length===40)break;}}
  generation++;editing=true;group.input.type='password';setSecret(secret);
  feedback.textContent='';changed();secretInput().focus();
 });
 const group=adornedField('E2E secret','','password',{actions:[reveal,copy,change,generate]});
 // Native password inputs strip line endings; textareas normalize CRLF.
 // Keep the exact secret separately, using a textarea only for revealed multiline text.
 const multiline=el('textarea');multiline.id=`${group.input.id}-multiline`;multiline.rows=3;multiline.hidden=true;
 multiline.autocomplete='off';multiline.spellcheck=false;group.control.append(multiline);
 let rawSecret='',renderedSecret='';
 function secretInput(){return multiline.hidden?group.input:multiline;}
 function secret(){const value=secretInput().value;if(value!==renderedSecret){rawSecret=value;renderedSecret=value;}return rawSecret;}
 function setSecret(value){
  rawSecret=value;
  const showMultiline=group.input.type==='text'&&/[\r\n]/.test(value);
  multiline.hidden=!showMultiline;group.input.hidden=showMultiline;
  group.input.value='';multiline.value='';
  const control=secretInput();control.value=value;renderedSecret=control.value;
  group.wrap.children[0].htmlFor=control.id;
  // Editing a concealed multiline value would use incorrect native cursor offsets.
  group.input.readOnly=!editing||(!showMultiline&&/[\r\n]/.test(value));
  multiline.readOnly=!editing;
 }
 function rawOffset(offset){
  if(multiline.hidden)return offset;
  let i=0,n=0;while(i<rawSecret.length&&n<offset){if(rawSecret[i]==='\r'&&rawSecret[i+1]==='\n')i++;i++;n++;}return i;
 }
 for(const control of [group.input,multiline])control.addEventListener('paste',event=>{
  if(!editing||saving||control.readOnly||!event.clipboardData)return;
  const text=event.clipboardData.getData('text/plain');if(!/[\r\n]/.test(text)&&multiline.hidden)return;
  event.preventDefault();const before=secret(),start=rawOffset(control.selectionStart??before.length),end=rawOffset(control.selectionEnd??before.length);
  group.input.type='text';setSecret(before.slice(0,start)+text+before.slice(end));changed();secretInput().focus();
 });
 multiline.addEventListener('copy',event=>{
  if(!event.clipboardData)return;secret();
  event.clipboardData.setData('text/plain',rawSecret.slice(rawOffset(multiline.selectionStart),rawOffset(multiline.selectionEnd)));
  event.preventDefault();
 });
 ctx.ui.withInfo(group,'At least 32 characters. The same secret links your Zelports, and they can administer each other.');group.wrap.append(feedback,copyConfirmation.root);
 const enabled=field('Enabled',false,'checkbox'),publish=field('Share my services',false,'checkbox');ctx.ui.withInfo(publish,'Linked Zelports can import all services of this Zelport.');
 // Remote file access belongs with the other choices about linked Zelports. It is saved with this form,
 // and only on this computer: a remote administration session sees it read-only.
 const fileAccess=field('Allow remote file access',false,'checkbox');ctx.ui.withInfo(fileAccess,'Lets Zelports that administer this one remotely change the SSH config and CA file paths and edit those files.');
 const fileRisk=el('p','field-hint field-warning remote-file-access-risk');fileRisk.append(ctx.ui.icon?.('warning',{className:'warning-glyph'})||el('span'),el('span',null,'Remote devices can run commands on this computer.'));fileRisk.hidden=true;
 const fileNote=el('p','small muted remote-file-access-note','Change this on the computer itself.');fileNote.hidden=true;
 const savedFileAccess=()=>!!ctx.ui.state()?.remote_file_access,remoteSession=()=>!!ctx.ui.state()?.remote_admin;
 const switches=el('div','relay-connection-options');switches.append(enabled.wrap,publish.wrap,fileAccess.wrap,fileRisk,fileNote);
 const error=el('p','route-error'),actions=el('div','relay-actions'),save=el('button','primary','Save');save.type='submit';
 const discard=button('Discard',()=>{if(saving)return;sync();validation.clear();url.input.focus();});
 // The connection state heads the card: dot, relay host, state (details in its tooltip).
 const status=el('div','relay-status-line');
 actions.append(saveConfirmation.root,discard,save);form.append(status,url.wrap,modeWrap,access.wrap,group.wrap,switches,error,actions);
 url.input.placeholder='ws://relay.example.com:8070';access.input.autocomplete=group.input.autocomplete='off';group.input.spellcheck=false;
 const validation=ctx.ui.formFeedback(form,error,[{input:url.input,matches:/url|scheme|wss?:/i},{input:access.input,matches:/access token/i},{get input(){return secretInput();},matches:/E2E|group token/i}]);
 function mask(){generation++;const value=editing?secret():'';group.input.type='password';setSecret(value);feedback.textContent='';update();}
 function setRevealLabel(){const label=group.input.type==='text'?'Hide E2E secret':'Reveal E2E secret';reveal.setAttribute('aria-label',label);setTooltip(reveal,label);}
 function update(){
  for(const input of [url.input,mode,access.input,group.input,multiline,enabled.input,publish.input])input.disabled=saving;
  fileAccess.input.disabled=saving||remoteSession();fileNote.hidden=!remoteSession();fileRisk.hidden=!fileAccess.input.checked;
  group.input.disabled=saving||!multiline.hidden;multiline.disabled=saving||multiline.hidden;
  access.wrap.hidden=mode.value!=='token';access.input.disabled=saving||mode.value!=='token';access.input.required=mode.value==='token'&&!saved.has_access_token;
  access.input.placeholder=saved.has_access_token?'Stored · leave blank to keep':'Paste relay access token';
  group.input.readOnly=!editing||/[\r\n]/.test(secret());multiline.readOnly=!editing;group.input.placeholder=editing?'Enter a shared secret or generate one':'Secret stored';
  group.input.required=editing&&multiline.hidden;multiline.required=editing&&!multiline.hidden;change.hidden=editing;generate.hidden=!editing;
  change.disabled=generate.disabled=saving;reveal.disabled=copy.disabled=saving||(editing?!secret():!saved.has_identity);
  setRevealLabel();save.disabled=discard.disabled=saving||!dirty;ctx.ui.busyButton(save,saving);form.setAttribute('aria-busy',String(saving));
 }
 function changed(){
  generation++;
  dirty=url.input.value!==(saved.relay_url||'')||mode.value!==(saved.has_access_token?'token':'none')||!!access.input.value||enabled.input.checked!==!!saved.enabled||publish.input.checked!==!!saved.publish||fileAccess.input.checked!==savedFileAccess()||(editing&&(saved.has_identity||!!secret()));
  feedback.textContent='';saveConfirmation.clear();update();
 }
 function sync(){
  generation++;dirty=false;editing=!saved.has_identity;url.input.value=saved.relay_url||'';mode.value=saved.has_access_token?'token':'none';access.input.value='';group.input.type='password';setSecret('');enabled.input.checked=!!saved.enabled;publish.input.checked=!!saved.publish;fileAccess.input.checked=savedFileAccess();feedback.textContent='';revision=ctx.ui.state().revision;update();
 }
 async function storedSecret(){
  const request=++generation,requestedRevision=ctx.ui.state().revision;
  try{
   const result=await ctx.api(`/api/modules/${ctx.moduleId}/credentials`,{method:'POST',body:JSON.stringify({revision:requestedRevision,action:'reveal'})});
   if(disposed||!active||editing||ctx.signal?.aborted||request!==generation||requestedRevision!==ctx.ui.state().revision)return;
   revealedRevision=requestedRevision;return result.group_token;
  }catch(cause){if(!disposed&&active&&request===generation)feedback.textContent=cause.message;}
 }
 async function revealSecret(){
  if(group.input.type==='text'){mask();return;}
  const value=editing?secret():await storedSecret();if(!value)return;
  group.input.type='text';setSecret(value);setRevealLabel();update();
 }
 async function copySecret(){
  const value=editing?secret():await storedSecret();if(!value)return;
  const request=generation,requestedRevision=ctx.ui.state().revision;
  const current=()=>active&&!disposed&&!ctx.signal?.aborted&&request===generation&&requestedRevision===ctx.ui.state().revision;
  try{await navigator.clipboard.writeText(value);if(current())copyConfirmation.show('Copied');}
  catch{if(current()){group.input.type='text';setSecret(value);setRevealLabel();update();secretInput().focus();secretInput().select();feedback.textContent='Select and copy the secret.';}}
 }
 form.addEventListener('input',changed);form.addEventListener('change',changed);
 form.addEventListener('submit',async event=>{
  event.preventDefault();if(saving||!dirty)return;validation.clear();
  if(editing&&!secret()){validation.show('Paste a shared secret or generate a new one.',secretInput());return;}
  if(editing&&[...secret()].length<32){validation.show('Use at least 32 characters for the E2E secret.',secretInput());return;}
  if(editing&&[...secret()].length>256){validation.show('Use no more than 256 characters for the E2E secret.',secretInput());return;}
  if(mode.value==='token'&&!saved.has_access_token&&!access.input.value.trim()){validation.show('Enter the relay access token.',access.input);return;}
  const body={revision,relay_url:url.input.value.trim(),enabled:enabled.input.checked,publish:publish.input.checked,access_token:mode.value==='token'?access.input.value.trim():'',clear_access_token:mode.value==='none',group_token:editing?secret():''};
  saving=true;generation++;update();let failure;
  try{
   const relayChanged=url.input.value!==(saved.relay_url||'')||mode.value!==(saved.has_access_token?'token':'none')||!!access.input.value||enabled.input.checked!==!!saved.enabled||publish.input.checked!==!!saved.publish||editing;
   const fileChanged=fileAccess.input.checked!==savedFileAccess();
   if(relayChanged||!fileChanged)await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/settings`,{method:'PUT',body:JSON.stringify(body)}),{reportError:false});
   if(fileChanged)await ctx.mutate(()=>ctx.api('/api/remote-file-access',{method:'PUT',body:JSON.stringify({revision:relayChanged?ctx.ui.state().revision:revision,enabled:fileAccess.input.checked})}),{reportError:false});
   saved={...saved,relay_url:body.relay_url,enabled:body.enabled,publish:body.publish,has_identity:true,has_access_token:mode.value==='token'};sync();saveConfirmation.show('Saved');
  }catch(cause){if(cause.status===409){revision=ctx.ui.state().revision;failure='Changed elsewhere. Your draft is kept. Save again or discard.';}else failure=cause.message.replace(/^ports_over_relay settings: /,'');}
  finally{saving=false;update();if(failure)validation.show(failure);}
 });
 return {element:form,status,render(state){
  const next=state.data||{},before=JSON.stringify([saved.enabled,saved.publish,saved.relay_url,saved.has_access_token,saved.has_identity]),after=JSON.stringify([next.enabled,next.publish,next.relay_url,next.has_access_token,next.has_identity]);
  saved=next;
  if(!dirty&&!saving){if(before!==after||!revision||fileAccess.input.checked!==savedFileAccess())sync();else revision=ctx.ui.state().revision;}
  update();
  if(!editing&&secret()&&revealedRevision!==state.revision)mask();
 },focus(){url.input.focus();},activate(){active=true;},deactivate(){active=false;mask();},dispose(){disposed=true;active=false;generation++;access.input.value='';setSecret('');}};
}
