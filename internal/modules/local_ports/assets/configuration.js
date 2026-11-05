const formatNames=names=>names.map(name=>(name.includes('"')||name.includes(','))?'"'+name.replaceAll('"','""')+'"':name).join(', ');
function parseNames(text){
 const names=[];let value='',quoted=false,closed=false;
 const invalid=()=>Error('Use comma-separated names. Quote names with commas.');
 const push=()=>{if(value.trim())names.push(value.trim());value='';closed=false;};
 for(let i=0;i<text.length;i++){
  const char=text[i];
  if(quoted){if(char==='"'){if(text[i+1]==='"'){value+='"';i++;}else{quoted=false;closed=true;}}else value+=char;}
  else if(char===',')push();
  else if(char==='"'){if(value.trim()||closed)throw invalid();value='';quoted=true;}
  else if(closed){if(char.trim())throw invalid();}
  else value+=char;
 }
 if(quoted)throw invalid();push();return names;
}

export function mountConfiguration(ctx,panel){
 const {el,field,button}=ctx.ui;
 let validation,current={},form,enabled,ports,processes,baseline='',revision,dirty=false,saving=false,disposed=false;
 const error=el('p','route-error'),feedback=ctx.ui.confirmation(),actions=el('div','local-settings-actions');error.setAttribute('role','alert');
 const save=el('button','primary','Save');save.type='submit';
 const discard=button('Discard',()=>{if(saving)return;sync();validation?.clear();feedback.clear();update();});actions.append(feedback.attach(save).root,discard,save);
 const signature=()=>JSON.stringify([current.discovery_enabled!==false,current.ignored_ports||[],current.ignored_processes||[]]);
 function sync(){enabled.input.checked=current.discovery_enabled!==false;ports.input.value=(current.ignored_ports||[]).join(', ');processes.input.value=formatNames(current.ignored_processes||[]);baseline=signature();revision=ctx.ui.state().revision;dirty=false;}
 function update(){if(!form)return;for(const control of [enabled,ports,processes])control.input.disabled=saving;save.disabled=discard.disabled=saving||!dirty;ctx.ui.busyButton(save,saving);form.setAttribute('aria-busy',String(saving));}
 function changed(){dirty=enabled.input.checked!==(current.discovery_enabled!==false)||ports.input.value!==(current.ignored_ports||[]).join(', ')||processes.input.value!==formatNames(current.ignored_processes||[]);feedback.clear();update();}
 function open(){
  if(disposed||form)return;
  form=el('form','local-configuration-form');enabled=field('Automatic port detection',true,'checkbox');ports=field('Ignored ports');processes=field('Ignored processes');
  ports.input.placeholder='22, 5432, 6379';processes.input.placeholder='sshd, postgres';
  ctx.ui.withInfo(enabled,'Suggests listening ports when you add a service.');ctx.ui.withInfo(ports,'Comma-separated ports or ranges.');ctx.ui.withInfo(processes,'Comma-separated, case-insensitive.');
  for(const control of [ports,processes]){control.input.autocomplete='off';control.input.spellcheck=false;}
  form.append(enabled.wrap,ports.wrap,processes.wrap,error,actions);
  for(const control of [enabled,ports,processes])control.input.addEventListener('input',changed);
  validation=ctx.ui.formFeedback(form,error,[{input:ports.input,matches:/port/i},{input:processes.input,matches:/process|comma|quote/i}]);
   form.addEventListener('submit',submit);panel.append(form);sync();update();
 }
 async function submit(event){
  event.preventDefault();if(disposed||saving||!dirty)return;saving=true;validation?.clear();feedback.clear();update();
  const list=text=>text.split(',').map(value=>value.trim()).filter(Boolean);
  try{
   await ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/settings`,{method:'PUT',body:JSON.stringify({revision,discovery_enabled:enabled.input.checked,ignored_ports:list(ports.input.value),ignored_processes:parseNames(processes.input.value)})}),{reportError:false});
   if(disposed)return;sync();feedback.show('Saved');
  }catch(cause){if(disposed)return;if(cause.status===409){revision=ctx.ui.state().revision;error.textContent='Changed elsewhere. Your draft is kept. Save again or discard.';}else error.textContent=cause.message;}
  finally{saving=false;if(!disposed){update();if(error.textContent)validation.show(error.textContent);}}
 }
 return {open,render(state){current=state.data||{};if(!form)return;if(!dirty&&!saving&&signature()!==baseline)sync();else if(signature()===baseline)revision=ctx.ui.state().revision;update();},dispose(){disposed=true;form?.remove();}};
}
