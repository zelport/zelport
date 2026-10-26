import {adornmentButton,button,el,emptyState,icon,renderChanged,serviceTargetIndicator,serviceURL,statusBadge,warningIndicator} from './ui-dom.js';
import {adornedField,formFeedback} from './ui-forms.js';
import {dialogHeader,disclosureMenu,formActions,showDialog} from './ui-dialogs.js';

export function routeEditRevision(original,state){if(original?.id){const latest=Object.values(state.states).flatMap(s=>s.routes||[]).find(r=>r.id===original.id);if(!latest)throw Error('Removed in another window.');const canonical=value=>JSON.stringify(value,(_,v)=>v&&typeof v==='object'&&!Array.isArray(v)?Object.fromEntries(Object.keys(v).sort().map(k=>[k,v[k]])):v),content=r=>canonical({module:r.module,domain:r.domain,editor:r.editor});if(content(original)!==content(latest)){const error=Error('Changed in another window. Your draft is kept. Reload to see the saved values.');error.currentRoute=latest;throw error;}}return state.revision;}
export function refreshControl(label,action){const control=adornmentButton(label,'rotate',action);control.className='module-action-icon refresh-icon';return control;}
export {refreshControl as refreshButton};

// Service forms open in one drawer. Switching the source swaps the drawer content in
// place, so its size, position and actions stay put; each content owns its cleanup.
const drawerContent=new WeakMap();
// Every close path slides the drawer out first. Escape and a click on the backdrop
// close it unless a save is in flight.
function drawerBehavior(d){
 const nativeClose=d.close.bind(d),busy=()=>!!d.querySelector('form[aria-busy="true"]');
 d.close=value=>{
  if(!d.open||d.dataset.state==='closing')return;
  if(!globalThis.matchMedia?.('(prefers-reduced-motion: no-preference)').matches){nativeClose(value);return;}
  d.dataset.state='closing';setTimeout(()=>{delete d.dataset.state;nativeClose(value);},180);
 };
 d.addEventListener('cancel',event=>{event.preventDefault();if(!busy())d.close();});
 const outside=event=>{const box=d.getBoundingClientRect();return event.target===d&&(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom);};
 let pressedOutside=false;
 d.addEventListener('pointerdown',event=>{pressedOutside=outside(event);});
 d.addEventListener('click',event=>{if(pressedOutside&&outside(event)&&!busy())d.close();pressedOutside=false;});
}
export function serviceModule(ctx,definition){let current={routes:[],data:{}},dialog=null;const getState=()=>ctx.ui.state();const base=`/api/modules/${ctx.moduleId}`;
 let refreshing=false,refreshButton=null;const menus=[];
 const disposeMenus=()=>{for(const menu of menus)menu.dispose();menus.length=0;};
 const updateRefresh=()=>{if(refreshButton){refreshButton.disabled=refreshing;refreshButton.setAttribute('aria-busy',String(refreshing));}};
 async function refreshDiscovery(){if(refreshing)return;refreshing=true;updateRefresh();try{await ctx.mutate(()=>ctx.api(`${base}/refresh`,{method:'POST',body:'{}'}));}finally{refreshing=false;updateRefresh();}}
 if(definition.refresh&&ctx.toolbar){refreshButton=refreshControl('Refresh',refreshDiscovery);ctx.toolbar.append(refreshButton);}
 const remove=route=>{
  const d=el('dialog'),title=dialogHeader('Remove service',d),error=el('p','route-error'),actions=el('div','dialog-actions'),endpoint=serviceURL(getState(),route)||`http://${route.domain}:${getState().port}`;
  let busy=false;const run=async work=>{if(busy)return;busy=true;feedback.clear();for(const control of actions.children)control.disabled=true;let failure;try{await work();d.close();}catch(e){failure=e.message;}finally{busy=false;for(const control of actions.children)control.disabled=false;if(failure)feedback.show(failure);}};
  actions.append(button('Cancel',()=>d.close()));
  for(const action of definition.removeActions?.(route)||[])actions.append(button(action.label,()=>run(action.run),action.className||'danger'));
  actions.append(button('Remove',()=>run(()=>ctx.mutate(()=>ctx.api(`/api/routes/${route.id}?revision=${getState().revision}`,{method:'DELETE'}),{reportError:false})),'danger'));
  d.append(title,el('p','remove-service-address',endpoint),el('p','small muted','The service itself keeps running.'),error,actions);
  const feedback=formFeedback(d,error);showDialog(d,{signal:ctx.signal});
 };
 function edit(route=null,group=null,options={}){
  const reuse=options.container?.open?options.container:null;
  if(dialog&&dialog!==reuse){dialog.close();dialog.remove();}
  const d=reuse||el('dialog','service-edit-dialog service-drawer');dialog=d;
  const form=el('form'),body=el('div','service-edit-body'),head=dialogHeader(route?.id?`Edit service${ctx.moduleName?` · ${ctx.moduleName}`:''}`:options.sources?'Add service':`${definition.createTitle||'Add service'}${ctx.moduleName?` · ${ctx.moduleName}`:''}`,d);
  form.append(head,body);
  const domain=adornedField('Address','','text',{suffix:`.localhost${getState().port===80?'':`:${getState().port}`}`}),{wrap:domainField,input}=domain,label=domainField.children[0];
  input.id=`domain-${ctx.moduleId}`;label.htmlFor=input.id;input.required=true;
  input.pattern='[A-Za-z0-9]([A-Za-z0-9\\-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9\\-]{0,61}[A-Za-z0-9])?)*';
  input.maxLength=243;input.value=route?.domain.replace(/\.localhost$/,'')||options.domain||suggest(group?.name||'service',!group?.name);
  // The suffix follows the typed name, so the address reads as one hostname.
  domain.control.className+=' address-adornment';
  const fitAddress=()=>{
   const style=globalThis.getComputedStyle?.(input);if(!style)return;
   const context=(fitAddress.canvas ||= document.createElement('canvas')).getContext?.('2d');if(!context)return;
   context.font=`${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
   const padding=parseFloat(style.paddingLeft)+parseFloat(style.paddingRight);
   input.style.width=`${Math.ceil(context.measureText(input.value||input.placeholder||'').width+padding+1)}px`;
  };
  input.addEventListener('input',fitAddress);
  domain.control.addEventListener('mousedown',event=>{if(event.target===input)return;if(event.target===domain.control||event.target.classList?.contains('input-adornment-text')){event.preventDefault();input.focus();input.setSelectionRange?.(input.value.length,input.value.length);}});
  if(options.sources){
   const choices=el('fieldset','service-source-choices');choices.append(el('legend',null,'Where does the service run?'));
   for(const source of options.sources){
    const choice=el('label','service-source-choice'),radio=el('input'),text=el('span','service-source-text');
    radio.type='radio';radio.name='service-source';radio.value=source.id;radio.checked=source.id===options.sourceId;radio.className='service-source-radio';
    text.append(el('strong',null,source.label),el('span','small muted',source.description));choice.append(radio);if(source.icon)choice.append(icon(source.icon,{className:'service-source-icon'}));choice.append(text);choices.append(choice);
    radio.addEventListener('change',()=>{if(!radio.checked||source.id===options.sourceId)return;options.onSource?.(source.id,{domain:input.value,container:d});});
   }
   body.append(choices);
  }
  body.append(domainField);
  const fields=definition.fields(route?.editor||{},group,current,{address:input,isNew:!route?.id});body.append(fields.element);
  let fieldsDisposed=false;const disposeFields=()=>{if(fieldsDisposed)return;fieldsDisposed=true;fields.dispose?.();};
  const {error,actions,save}=formActions(d,{submit:route?.id?'Save':definition.createSubmit||'Add service'});
  let changedRoute=null;
  const reload=button('Reload saved values',()=>{if(changedRoute)edit(changedRoute,group,{...options,container:d});});reload.hidden=true;
  body.append(error,reload);form.append(actions);
  const feedback=formFeedback(form,error,[{input,matches:/domain/i}]);
  input.addEventListener('invalid',event=>{event.preventDefault();feedback.show(input.validity.valueMissing?'Enter an address.':'Use letters, numbers, hyphens and dots.',input);});
  form.addEventListener('submit',async event=>{
   event.preventDefault();if(save.disabled)return;
   const controls=[...d.querySelectorAll('button,input,select,textarea')],disabled=controls.map(control=>!!control.disabled);
   for(const control of controls)control.disabled=true;
   save.disabled=true;form.setAttribute('aria-busy','true');feedback.clear();reload.hidden=true;changedRoute=null;let revision,failure,failureInput;
   try{
    const settings=fields.read();revision=routeEditRevision(route,getState());
    await ctx.mutate(()=>ctx.api('/api/routes',{method:'POST',body:JSON.stringify({revision,route:{id:route?.id||'',module:ctx.moduleId,domain:input.value.toLowerCase()+'.localhost',settings}})}),{reportError:false});
    if(!ctx.signal?.aborted&&d.open&&d.dataset.state!=='closing'){d.close();options.onSaved?.();}
   }catch(cause){
    if(cause.status===409){try{const latestRevision=routeEditRevision(route,getState());cause=Error(latestRevision!==revision?'Changed elsewhere. Your draft is kept. Save again or discard.':'This address is already in use.');}catch(conflict){cause=conflict;}}
    failure=cause.message;failureInput=cause.input;changedRoute=cause.currentRoute||null;reload.hidden=!changedRoute;
   }finally{
    controls.forEach((control,index)=>{control.disabled=disabled[index];});save.disabled=false;form.setAttribute('aria-busy','false');if(failure&&d.open)feedback.show(failure,failureInput);
   }
  });
  const release=()=>{disposeFields();if(dialog===d)dialog=null;};
  if(reuse){
   drawerContent.get(d)?.();drawerContent.set(d,release);d.replaceChildren(form);fitAddress();
   form.querySelector('.service-source-radio:checked')?.focus();return d;
  }
  drawerContent.set(d,release);d.append(form);drawerBehavior(d);
  const shown=showDialog(d,{signal:ctx.signal,initialFocus:input,onClose(){drawerContent.get(d)?.();drawerContent.delete(d);if(options.trigger?.isConnected)options.trigger.focus();}});
  fitAddress();globalThis.document?.fonts?.ready?.then(fitAddress);return shown;
 }
 function suggest(value,numbered=false){let base=value.toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'')||'service';base=base.slice(0,50);const used=new Set(Object.values(getState().states).flatMap(s=>(s.routes||[]).map(r=>r.domain)));let n=numbered?1:2,name=numbered?`${base}-${n++}`:base;while(used.has(name+'.localhost'))name=`${base}-${n++}`;return name;}
 function row(route){const tabular=!!definition.columns,box=el('div','service'),name=el('div'),address=serviceURL(getState(),route),link=el(address?'a':'span',null,definition.serviceLabel?.(route,getState())||route.domain);if(tabular){box.setAttribute('role','row');name.setAttribute('role','cell');}if(address){link.href=address;link.target='_blank';link.rel='noopener';}name.append(link);if(!address)name.append(el('span','small muted','Not connected on this device'));if(!tabular&&route.summary.detail&&route.module!=='remote_http')name.append(el('span','small route-error',route.summary.detail));const target=el('div','target-address');if(tabular){target.setAttribute('role','cell');target.append(el('span','mobile-target-label',`${definition.mobileTargetLabel||definition.columns[1]} `));}target.append(el('span',null,definition.targetLabel?.(route)||route.summary.target));if(!tabular&&route.module==='remote_http'&&route.summary.detail)target.append(warningIndicator(route.summary.detail).root);const actions=el('div','service-actions');if(tabular)actions.setAttribute('role','cell');const indicators=definition.rowIndicators?.(route)||[];let details=null;if(tabular){details=el('div','service-port-details');details.setAttribute('role','cell');}if(indicators.length||definition.targetInfo){const group=el('span','service-indicators');group.append(...indicators);if(definition.targetInfo)group.append(serviceTargetIndicator(definition.targetInfo(route),route.domain).root);(details||actions).append(group);}if(definition.routeMenu){const root=el('div','resource-actions service-actions-menu'),trigger=el('button','resource-menu-trigger service-menu-trigger','⋮'),menu=el('div','resource-menu service-menu');trigger.setAttribute('aria-label',`Actions for ${route.domain}`);trigger.setAttribute('aria-haspopup','menu');menu.setAttribute('role','menu');root.append(trigger,menu);const disclosure=disclosureMenu(root,trigger,menu);menus.push(disclosure);const editAction=button('Edit',()=>{disclosure.close(true);edit(route);},'resource-menu-item'),removeAction=button('Remove',()=>{disclosure.close(true);remove(route);},'resource-menu-item service-menu-remove');editAction.setAttribute('role','menuitem');removeAction.setAttribute('role','menuitem');menu.append(editAction,removeAction);actions.append(root);}else actions.append(button('Edit',()=>edit(route)),button('Remove',()=>remove(route),'danger-text'));for(const action of definition.routeActions?.(route)||[])actions.append(button(action.label,action.run));box.append(name,target,...(details?[details]:[]),actions);return box;}
 function render(s){current=s;const {updated_at,...data}=s.data||{},globalState=getState(),groups=definition.groups?.(s)||null,seen=new Set();if(groups)for(const group of groups)for(const route of s.routes)if(definition.matches(route,group))seen.add(route.id);const orphans=groups?s.routes.filter(route=>!seen.has(route.id)):[],hasEmpty=groups?!groups.length&&!orphans.length:!s.routes.length,empty=hasEmpty?(typeof definition.emptyState==='function'?definition.emptyState(s):definition.emptyState)||{kind:groups?'no-results':'not-configured',message:groups?'No resources found.':definition.empty||'No services yet.',actionLabel:definition.add===false?undefined:'+ Add service'}:null,emptyKey=empty&&{kind:empty.kind,message:empty.message,actionLabel:empty.actionLabel},content=(definition.hideServicesOnDiscoveryError&&!definition.statuses&&data.error?{error:data.error}:{routes:s.routes,error:data.error,groups,statuses:definition.statuses?.(s),empty:emptyKey,port:globalState.port,remote_admin:globalState.remote_admin,rowState:definition.rowState?.(s)});renderChanged(ctx.panel,content,()=>{const opened=new Set([...ctx.panel.querySelectorAll('details[open]')].map(d=>d.dataset.key));disposeMenus();ctx.panel.replaceChildren();if(definition.refresh){const header=el('div','refresh-row');for(const item of definition.statuses?.(s)||[]){const label=`${item.name} ${item.status}`;if(item.error){const detail=el('details','source-status source-status-error');detail.dataset.key=`status:${item.key}`;detail.open=opened.has(detail.dataset.key);detail.append(el('summary',null,label),el('div','source-status-detail',item.error));header.append(detail);}else header.append(el('span','source-status muted',label));}if(header.childElementCount)ctx.panel.append(header);}if(s.data?.error){if(definition.discoveryErrorTitle){const box=el('section','discovery-error');box.append(el('strong','route-error',definition.discoveryErrorTitle));const detail=el('details');detail.dataset.key='discovery-error';detail.open=opened.has(detail.dataset.key);detail.append(el('summary',null,'CLI output'),el('pre',null,s.data.error));box.append(detail);ctx.panel.append(box);}else ctx.panel.append(el('p','route-error',s.data.error));if(definition.hideServicesOnDiscoveryError)return;}if(groups){for(const group of groups){const details=el('details','resource');details.dataset.key=group.key;details.open=opened.has(group.key);const summary=el('summary');summary.append(el('strong',null,group.name));if(group.source)summary.append(el('span','small muted',group.source));summary.append(statusBadge(group.status));const groupActions=definition.groupActions?.(group)||[];if(groupActions.length){const root=el('div','resource-actions'),toggle=el('button','resource-menu-trigger','⋮'),menu=el('div','resource-menu');toggle.setAttribute('aria-label',`Actions for ${group.name}`);root.append(toggle,menu);const disclosure=disclosureMenu(root,toggle,menu);menus.push(disclosure);for(const action of groupActions)menu.append(button(action.label,()=>{disclosure.close(true);return action.run();},'resource-menu-item'));summary.append(root);}const services=el('div','services');for(const route of s.routes.filter(route=>definition.matches(route,group)))services.append(row(route));services.append(button('+ Add service',()=>edit(null,group)));details.append(summary,services);ctx.panel.append(details);}if(orphans.length){ctx.panel.append(el('h2',null,'Unavailable services'));const orphanList=el('div','local-service-list');for(const route of orphans)orphanList.append(row(route));ctx.panel.append(orphanList);}if(!groups.length&&!orphans.length)ctx.panel.append(emptyState(empty));}else if(!s.routes.length)ctx.panel.append(emptyState({...empty,action:empty?.actionLabel?{label:empty.actionLabel,run:()=>edit()}:undefined}));else{const area=el('div','local-service-list');if(definition.columns){area.setAttribute('role','table');area.setAttribute('aria-label',definition.tableLabel||'Services');const header=el('div','service-list-header');header.setAttribute('role','row');for(const label of definition.columns){const cell=el('span',null,label);cell.setAttribute('role','columnheader');header.append(cell);}area.append(header);}for(const route of s.routes)area.append(row(route));ctx.panel.append(area);}});}
 return {render,edit,remove,activate(){},deactivate(){for(const menu of menus)menu.close();},dispose(){disposeMenus();refreshButton?.remove();dialog?.close();dialog?.remove();}};
}
