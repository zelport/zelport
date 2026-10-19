// Render keys describe displayed content, never configuration freshness.
const renderedViews=new WeakMap();
export function setText(node,value){const text=String(value??'');if(node.textContent!==text)node.textContent=text;}
export function renderChanged(node,content,draw){const key=JSON.stringify(content);if(renderedViews.get(node)===key)return;draw();renderedViews.set(node,key);}
let fieldSequence=0;
// Adornments are siblings of the input so action buttons never activate its label.
export function inputAdornment(input,{prefix='',suffix='',startActions=[],actions=[]}={}){
 const root=el('div','input-adornment');
 if(startActions.length){const start=el('div','input-adornment-actions');start.append(...startActions);root.append(start);}
 if(prefix)root.append(el('span','input-adornment-text',prefix));
 root.append(input);
 if(suffix)root.append(el('span','input-adornment-text',suffix));
 if(actions.length){const end=el('div','input-adornment-actions');end.append(...actions);root.append(end);}
 return root;
}
export function adornedField(label,value='',type='text',adornments={}){
 const wrap=el('div','form-field'),caption=el('label',null,label),input=el('input');
 input.id=`field-${++fieldSequence}`;caption.htmlFor=input.id;input.type=type;input.value=value;
 const control=inputAdornment(input,adornments);wrap.append(caption,control);return {wrap,input,control};
}
const iconPaths={
 eye:'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Zm10-3a3 3 0 1 0 0 6 3 3 0 0 0 0-6',
 copy:'M9 9h11v12H9zM15 9V3H3v12h6',
 rotate:'M20 7v5h-5M20 12a8 8 0 1 0-2 6',
 experimental:'M9 3h6M10 3v7l-5 8a2 2 0 0 0 2 3h10a2 2 0 0 0 2-3l-5-8V3M8 15h8'
};
export function icon(name,{className='',title=''}={}){
 if(!iconPaths[name])throw new Error(`Unknown UI icon: ${name}`);
 const svg=document.createElementNS('http://www.w3.org/2000/svg','svg'),path=document.createElementNS('http://www.w3.org/2000/svg','path');
 svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('aria-hidden','true');svg.setAttribute('focusable','false');
 if(className)svg.setAttribute('class',className);
 if(title){const caption=document.createElementNS('http://www.w3.org/2000/svg','title');caption.textContent=title;svg.append(caption);}
 path.setAttribute('d',iconPaths[name]);svg.append(path);return svg;
}
export function adornmentButton(label,name,fn){
 const control=button('',fn,'adornment-button');control.setAttribute('aria-label',label);control.title=label;control.append(icon(name));return control;
}
// Global viewport listeners retain no row nodes after a render.
let warningViewportListeners=false;
function dismissWarningTooltips(){
 for(const tip of document.querySelectorAll('.warning-tooltip:popover-open')){tip.hidePopover();tip.hidden=true;}
}
export function warningIndicator(message=''){
 const root=el('span','service-warning'),trigger=el('button','warning-trigger'),tip=el('span','warning-tooltip');
 const glyph=el('span','warning-glyph','!');glyph.setAttribute('aria-hidden','true');trigger.append(glyph);
 trigger.type='button';trigger.setAttribute('aria-label','Connection security warning');
 tip.id=`warning-${++fieldSequence}`;tip.setAttribute('role','tooltip');tip.setAttribute('popover','manual');tip.hidden=true;
 trigger.setAttribute('aria-describedby',tip.id);
 let hovered=false,focused=false;
 const show=()=>{
  if(root.hidden)return;
  if(!warningViewportListeners){
   window.addEventListener('scroll',dismissWarningTooltips,true);
   window.addEventListener('resize',dismissWarningTooltips);
   warningViewportListeners=true;
  }
  tip.hidden=false;tip.showPopover();
  const anchor=trigger.getBoundingClientRect(),bounds=tip.getBoundingClientRect();
  tip.style.left=`${Math.max(16,Math.min(anchor.left-32,window.innerWidth-bounds.width-16))}px`;
  const above=anchor.top-bounds.height-8;
  tip.style.top=`${Math.max(16,Math.min(above>=16?above:anchor.bottom+8,window.innerHeight-bounds.height-16))}px`;
 },close=()=>{if(!tip.hidden)tip.hidePopover();tip.hidden=true;};
 root.addEventListener('mouseenter',()=>{hovered=true;show();});
 root.addEventListener('mouseleave',()=>{hovered=false;if(!focused)close();});
 trigger.addEventListener('focus',()=>{focused=true;show();});
 trigger.addEventListener('blur',()=>{focused=false;if(!hovered)close();});
 trigger.addEventListener('click',e=>{e.preventDefault();show();});
 root.addEventListener('keydown',e=>{if(e.key==='Escape'){e.preventDefault();e.stopPropagation();close();}});
 root.append(trigger,tip);
 const update=text=>{setText(tip,text);root.hidden=!text;if(!text)close();};
 update(message);return {root,update};
}
export function serviceURL(state,route){if(route.summary?.state==='unsupported')return null;return state.remote_admin?state.remote_admin.service_links?.[route.id]||null:`http://${route.domain}:${state.port}`;}
export function el(tag,className,text){const n=document.createElement(tag);if(className)n.className=className;if(text!==undefined)n.textContent=text;return n;}
export function button(text,fn,className='secondary'){const b=el('button',className,text);b.type='button';b.addEventListener('click',e=>{e.preventDefault();Promise.resolve(fn(e)).catch(()=>{});});return b;}
export function field(label,value='',type='text'){return type==='checkbox'?switchField(label,!!value):adornedField(label,value??'',type);}
// Menus share interaction mechanics; callers retain navigation/listbox semantics.
const openDisclosures=new Set();
export function disclosureMenu(root,trigger,menu,{selected=()=>false}={}){
 if(!menu.id)menu.id=`menu-${++fieldSequence}`;
 trigger.type='button';trigger.setAttribute('aria-controls',menu.id);trigger.setAttribute('aria-expanded','false');menu.hidden=true;
 const items=()=>[...menu.children].filter(n=>!n.hidden&&!n.disabled);
 let owner=null,pointerInteraction=false;const ownerClosed=()=>close();
 const pointerDown=()=>{pointerInteraction=true;},pointerCancel=()=>{pointerInteraction=false;};
 const outside=e=>{pointerInteraction=false;if(!root.contains(e.target))close();};
 const focusOutside=e=>{if(!pointerInteraction&&!root.contains(e.target))close();};
 function close(restore=false){
  openDisclosures.delete(close);
  menu.hidden=true;trigger.setAttribute('aria-expanded','false');
  owner?.removeEventListener('close',ownerClosed);owner=null;
  document.removeEventListener('pointerdown',pointerDown);document.removeEventListener('pointercancel',pointerCancel);document.removeEventListener('click',outside);document.removeEventListener('focusin',focusOutside);pointerInteraction=false;
  if(restore)trigger.focus();
 }
 function open(keyboard=false,last=false){
  if(trigger.disabled)return;
  for(const dismiss of openDisclosures)if(dismiss!==close)dismiss();
  openDisclosures.add(close);
  menu.hidden=false;trigger.setAttribute('aria-expanded','true');
  owner=root.closest?.('dialog');owner?.addEventListener('close',ownerClosed);
  document.addEventListener('pointerdown',pointerDown);document.addEventListener('pointercancel',pointerCancel);document.addEventListener('click',outside);document.addEventListener('focusin',focusOutside);
  const list=items();if(keyboard)(last?list.at(-1):(list.find(selected)||list[0]))?.focus();
 }
 const click=e=>{e.preventDefault();if(menu.hidden)open(e.detail===0);else close();};
 const triggerKeys=e=>{
  if(e.key==='Escape'&&!menu.hidden){e.preventDefault();e.stopPropagation();close(true);}
  if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();open(true,e.key==='ArrowUp');}
 };
 const menuKeys=e=>{
  if(e.key==='Escape'){e.preventDefault();e.stopPropagation();close(true);return;}
  if(!['ArrowDown','ArrowUp','Home','End'].includes(e.key))return;
  e.preventDefault();const list=items(),i=list.indexOf(document.activeElement);
  list[e.key==='Home'?0:e.key==='End'?list.length-1:(i+(e.key==='ArrowDown'?1:list.length-1))%list.length]?.focus();
 };
 trigger.addEventListener('click',click);trigger.addEventListener('keydown',triggerKeys);menu.addEventListener('keydown',menuKeys);
 return {open,close,select(){close(menu.contains(document.activeElement));},dispose(){close();trigger.removeEventListener('click',click);trigger.removeEventListener('keydown',triggerKeys);menu.removeEventListener('keydown',menuKeys);}};
}
export function picker(label,options,value,{placeholder='Select…'}={}){
 const wrap=el('div','mapping-fields'),caption=el('label',null,label),root=el('div','mapping-dropdown'),trigger=el('button','mapping-trigger'),menu=el('div','mapping-menu');
 trigger.id=`picker-${++fieldSequence}`;caption.htmlFor=trigger.id;caption.id=`${trigger.id}-label`;
 trigger.setAttribute('aria-haspopup','listbox');menu.setAttribute('role','listbox');menu.setAttribute('aria-labelledby',caption.id);
 let current=value;const result={wrap,get value(){return current},set disabled(value){trigger.disabled=value||!options.length;for(const child of menu.children)child.disabled=value;},onchange:null};
 const disclosure=disclosureMenu(root,trigger,menu,{selected:n=>n.dataset.value===current});
 function select(v,notify=false){
  current=v;setText(trigger,options.find(o=>o.value===v)?.label||placeholder);disclosure.close();
  for(const child of menu.children)child.setAttribute('aria-selected',String(child.dataset.value===v));
  if(notify){result.onchange?.(v);root.dispatchEvent(new Event('change',{bubbles:true}));}
 }
 for(const option of options){const control=button(option.label,()=>{select(option.value,true);trigger.focus();},'mapping-option');control.dataset.value=option.value;control.setAttribute('role','option');menu.append(control);}
 trigger.disabled=!options.length;root.append(trigger,menu);wrap.append(caption,root);select(value);
 // Closing an owning dialog releases listeners even while its picker is open.
 result.dispose=disclosure.dispose;return result;
}
// Rebase only over unrelated edits. Never silently overwrite or recreate a service.
export function routeEditRevision(original,state){
 if(original?.id){
  const latest=Object.values(state.states).flatMap(s=>s.routes||[]).find(r=>r.id===original.id);
  if(!latest)throw Error('This service was removed in another window. Your entries are preserved. Close this dialog to continue.');
  const canonical=value=>JSON.stringify(value,(_,v)=>v&&typeof v==='object'&&!Array.isArray(v)?Object.fromEntries(Object.keys(v).sort().map(k=>[k,v[k]])):v);
  const content=r=>canonical({module:r.module,domain:r.domain,editor:r.editor});
  if(content(original)!==content(latest)){
   const error=Error('This service changed in another window. Your entries are preserved. Reload the saved values to review the current service. Reloading discards your draft.');
   error.currentRoute=latest;throw error;
  }
 }
 return state.revision;
}
export function serviceModule(ctx,definition){let current={routes:[],data:{}},dialog=null;const getState=()=>ctx.ui.state();const base=`/api/modules/${ctx.moduleId}`;
 let refreshing=false,refreshButton=null;const menus=[];
 const disposeMenus=()=>{for(const menu of menus)menu.dispose();menus.length=0;};
 const updateRefresh=()=>{if(refreshButton){refreshButton.disabled=refreshing;refreshButton.setAttribute('aria-busy',String(refreshing));}};
 async function refreshDiscovery(){if(refreshing)return;refreshing=true;updateRefresh();try{await ctx.mutate(()=>ctx.api(`${base}/refresh`,{method:'POST',body:'{}'}));}finally{refreshing=false;updateRefresh();}}
 if(definition.refresh&&ctx.toolbar){refreshButton=refreshControl('Refresh',refreshDiscovery);ctx.toolbar.append(refreshButton);}
 const remove=route=>{
  const d=el('dialog'),title=dialogHeader('Remove service',d),error=el('p','route-error'),options=el('div','remove-options');
  error.setAttribute('role','alert');let busy=false;
  const run=async work=>{if(busy)return;busy=true;error.textContent='';for(const control of options.children)control.disabled=true;
   try{await work();d.close();}catch(e){error.textContent=e.message;}finally{busy=false;for(const control of options.children)control.disabled=false;}
  };
  options.append(button('Remove from proxy',()=>run(()=>ctx.mutate(()=>ctx.api(`/api/routes/${route.id}?revision=${getState().revision}`,{method:'DELETE'}),{reportError:false}))));
  for(const action of definition.removeActions?.(route)||[])options.append(button(action.label,()=>run(action.run),action.className||'danger'));
  options.append(button('Cancel',()=>d.close(),'text-button'));
  d.append(title,el('p',null,route.domain),error,options);showDialog(d,{signal:ctx.signal});
 };

 function edit(route=null,group=null){
  if(dialog){dialog.close();dialog.remove();}
  const d=el('dialog');dialog=d;const form=el('form'),head=dialogHeader(route?'Edit service':'Add a service',d);
  form.append(head);
  const label=el('label',null,'Domain name'),input=el('input'),domain=inputAdornment(input,{prefix:'http://',suffix:`.localhost:${getState().port}`});
  input.id=`domain-${ctx.moduleId}`;label.htmlFor=input.id;input.required=true;
  input.pattern='[A-Za-z0-9]([A-Za-z0-9\\-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9\\-]{0,61}[A-Za-z0-9])?)*';
  input.maxLength=243;input.value=route?.domain.replace(/\.localhost$/,'')||suggest(group?.name||'service');
  form.append(label,domain);
  const fields=definition.fields(route?.editor||{},group,current);form.append(fields.element);
  const {error,actions,save}=formActions(d,{submit:'Save service',draft:true,form});
  let changedRoute=null;
  const reload=button('Reload saved values',()=>{if(changedRoute)edit(changedRoute,group);});
  reload.hidden=true;form.append(error,reload,actions);
  form.addEventListener('submit',async e=>{
   e.preventDefault();if(save.disabled)return;save.disabled=true;error.textContent='';reload.hidden=true;changedRoute=null;
   let revision;
   try{
    const settings=fields.read();revision=routeEditRevision(route,getState());
    await ctx.mutate(()=>ctx.api('/api/routes',{method:'POST',body:JSON.stringify({revision,route:{id:route?.id||'',module:ctx.moduleId,domain:input.value.toLowerCase()+'.localhost',settings}})}),{reportError:false});
    if(!ctx.signal?.aborted)d.close();
   }catch(e){
    if(e.status===409){
     try{
      const latestRevision=routeEditRevision(route,getState());
      e=Error(latestRevision!==revision?'Configuration changed elsewhere. Your entries are preserved. Save again to retry.':'This domain is already in use. Choose another domain name.');
     }catch(conflict){e=conflict;}
    }
    error.textContent=e.message;changedRoute=e.currentRoute||null;reload.hidden=!changedRoute;
   }finally{save.disabled=false;}
  });
  d.append(form);showDialog(d,{signal:ctx.signal,initialFocus:input,onClose(){if(dialog===d)dialog=null;}});
 }
 function suggest(value){let base=value.toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'')||'service';base=base.slice(0,50);const used=new Set(Object.values(getState().states).flatMap(s=>(s.routes||[]).map(r=>r.domain)));let name=base,n=2;while(used.has(name+'.localhost'))name=`${base}-${n++}`;return name;}
 function row(route){const box=el('div','service'),name=el('div'),address=serviceURL(getState(),route),link=el(address?'a':'span',null,route.domain);if(address){link.href=address;link.target='_blank';link.rel='noopener';}name.append(link);if(!address)name.append(el('span','small muted','Not connected on this device'));if(route.summary.detail&&route.module!=='remote_http')name.append(el('span','small route-error',route.summary.detail));const actions=el('div','service-actions');actions.append(button('Edit',()=>edit(route)),button('Remove',()=>remove(route),'text-button'));for(const action of definition.routeActions?.(route)||[])actions.append(button(action.label,action.run));const target=el('div','target-address');target.append(el('span',null,route.summary.target));if(route.module==='remote_http'&&route.summary.detail)target.append(warningIndicator(route.summary.detail).root);box.append(name,target,actions);return box;}
 function render(s){current=s;
 // Revision and discovery timestamps are not displayed. Keep unchanged views attached.
 const {updated_at,...data}=s.data||{},globalState=getState();
 const content=(definition.hideServicesOnDiscoveryError&&!definition.statuses&&data.error
  ?{error:data.error}:{routes:s.routes,error:data.error,groups:definition.groups?.(s),statuses:definition.statuses?.(s),port:globalState.port,remote_admin:globalState.remote_admin});
 renderChanged(ctx.panel,content,()=>{
 const opened=new Set([...ctx.panel.querySelectorAll('details[open]')].map(d=>d.dataset.key));disposeMenus();ctx.panel.replaceChildren();if(definition.refresh){const header=el('div','refresh-row');for(const item of definition.statuses?.(s)||[]){const label=`${item.name} ${item.status}`;if(item.error){const detail=el('details','source-status source-status-error');detail.dataset.key=`status:${item.key}`;detail.open=opened.has(detail.dataset.key);detail.append(el('summary',null,label),el('div','source-status-detail',item.error));header.append(detail);}else header.append(el('span','source-status muted',label));}if(header.childElementCount)ctx.panel.append(header);}if(s.data?.error){if(definition.discoveryErrorTitle){const box=el('section','discovery-error');box.append(el('strong','route-error',definition.discoveryErrorTitle));const detail=el('details');detail.dataset.key='discovery-error';detail.open=opened.has(detail.dataset.key);detail.append(el('summary',null,'CLI output'),el('pre',null,s.data.error));box.append(detail);ctx.panel.append(box);}else ctx.panel.append(el('p','route-error',s.data.error));if(definition.hideServicesOnDiscoveryError){return;}}if(definition.groups){const groups=definition.groups(s),seen=new Set();for(const g of groups){const details=el('details','resource');details.dataset.key=g.key;details.open=opened.has(g.key);const summary=el('summary');summary.append(el('strong',null,g.name));if(g.source)summary.append(el('span','small muted',g.source));summary.append(statusBadge(g.status));const groupActions=definition.groupActions?.(g)||[];if(groupActions.length){const root=el('div','resource-actions'),toggle=el('button','resource-menu-trigger','⋮'),menu=el('div','resource-menu');toggle.setAttribute('aria-label',`Actions for ${g.name}`);root.append(toggle,menu);const disclosure=disclosureMenu(root,toggle,menu);menus.push(disclosure);for(const action of groupActions)menu.append(button(action.label,()=>{disclosure.close(true);return action.run();},'resource-menu-item'));summary.append(root);}const services=el('div','services');for(const r of s.routes.filter(r=>definition.matches(r,g))){seen.add(r.id);services.append(row(r));}services.append(button('+ Add service',()=>edit(null,g))); details.append(summary,services);ctx.panel.append(details);}const orphans=s.routes.filter(r=>!seen.has(r.id));if(orphans.length){ctx.panel.append(el('h2',null,'Unavailable services'));const orphanList=el('div','local-service-list');for(const r of orphans)orphanList.append(row(r));ctx.panel.append(orphanList);}if(!groups.length&&!orphans.length)ctx.panel.append(el('div','empty','No resources found.'));}else{const area=el('div',s.routes.length?'local-service-list':'empty');if(!s.routes.length)area.append(el('p',null,definition.empty||'No services yet.'));for(const route of s.routes)area.append(row(route));if(definition.add!==false)area.append(button('+ Add service',()=>edit()));ctx.panel.append(area);}});}
 return {render,edit,activate(){},deactivate(){for(const menu of menus)menu.close();},dispose(){disposeMenus();refreshButton?.remove();dialog?.close();dialog?.remove();}};
}

export function switchField(label,checked=false){
 const wrap=el('div','switch-field'),input=el('input'),caption=el('label',null,label);
 input.type='checkbox';input.checked=checked;input.id=`switch-${++fieldSequence}`;input.setAttribute('role','switch');caption.htmlFor=input.id;
 wrap.append(caption,input);return {wrap,input,caption};
}
export function accordion(title,maturity){
 const root=el('details','settings-accordion'),summary=el('summary'),content=el('div','settings-accordion-content');
 summary.append(el('span','settings-accordion-title',title));
 if(maturity==='experimental')summary.append(experimentalBadge());
 root.append(summary,content);return {root,summary,content};
}

// Shared status semantics never equate a configured route with verified health.
export function statusTone(value){
 const state=String(value||'').toLowerCase();
 if(['ready','running','listening','connected','available'].includes(state))return 'ok';
 if(['error','failed','unavailable'].includes(state))return 'error';
 if(['stale','waiting','connecting','pending'].includes(state))return 'pending';
 return 'neutral';
}
export function statusBadge(value){const badge=el('span','badge status-badge',value);badge.dataset.tone=statusTone(value);return badge;}
export function dialogClose(dialog){const close=button('×',()=>dialog.close(),'icon-button');close.setAttribute('aria-label','Close dialog');close.title='Close';return close;}
export function dialogHeader(title,dialog){
 const header=el('div','dialog-header'),heading=el('h2',null,title);
 heading.id=`dialog-title-${++fieldSequence}`;dialog.setAttribute('aria-labelledby',heading.id);
 header.append(heading,dialogClose(dialog));return header;
}
// The module signal owns dialog lifetime; native close restores the opener's focus.
export function showDialog(dialog,{initialFocus,onClose=()=>{},signal}={}){
 let closed=false;const abort=()=>dialog.close();
 const cleanup=()=>{if(closed)return;closed=true;signal?.removeEventListener('abort',abort);try{onClose();}finally{dialog.remove();}};
 if(signal?.aborted){cleanup();return dialog;}
 dialog.addEventListener('close',cleanup,{once:true});signal?.addEventListener('abort',abort,{once:true});
 // Reveal invalid fields before native validation tries to focus them.
 dialog.addEventListener('invalid',event=>{
  for(let node=event.target.parentElement;node&&node!==dialog;node=node.parentElement)if(node.tagName==='DETAILS')node.open=true;
 },true);
 document.body.append(dialog);dialog.showModal();initialFocus?.focus();return dialog;
}
export function formActions(dialog,{submit='Save',className='',draft=false,form}={}){
 const actions=el('div','dialog-actions'),cancel=button('Cancel',()=>dialog.close()),save=el('button',className,submit),error=el('p','route-error');
 save.type='submit';error.setAttribute('role','alert');actions.append(cancel,save);
 if(draft)draftNotice(form,actions);return {actions,cancel,save,error};
}
export function draftNotice(form,actions){
 const notice=el('span','draft-notice','Unsaved changes');notice.hidden=true;notice.setAttribute('role','status');actions.prepend(notice);
 const update=()=>{notice.hidden=false;};form.addEventListener('input',update);form.addEventListener('change',update);
 return notice;
}
export function refreshControl(label,action){const control=adornmentButton(label,'rotate',action);control.className='refresh-icon';return control;}

export {refreshControl as refreshButton};

// Group the existing module controls without remounting their panels.
export function serviceNavigation(tabs){
 const items=[...tabs.children],dashboard=items.find(n=>n.id==='tab-dashboard'),settings=items.find(n=>n.id==='tab-settings');
 const group=el('div','services-navigation'),menu=el('div','services-menu'),trigger=el('button','services-trigger','Services ▾');
 menu.id='services-navigation-menu';menu.setAttribute('role','group');menu.setAttribute('aria-label','Service sources');
 const label=item=>item.dataset.navigationLabel||item.textContent||'';
 const sources=items.filter(item=>item!==dashboard&&item!==settings).sort((a,b)=>label(a).localeCompare(label(b),'en',{sensitivity:'base'}));
 for(const item of sources)menu.append(item);
 group.append(trigger,menu);tabs.append(group,settings);tabs.setAttribute('role','navigation');tabs.className='primary-navigation';
 const selected=n=>n.getAttribute('aria-current')==='page';
 const disclosure=disclosureMenu(group,trigger,menu,{selected});
 function update(){
  const visible=[...menu.children].filter(n=>!n.hidden);
  trigger.disabled=!visible.length;trigger.title=trigger.disabled?'No service modules available':'';
  if(trigger.disabled)disclosure.close();trigger.dataset.selected=String(visible.some(selected));
 }
 const changes=new MutationObserver(update);changes.observe(menu,{subtree:true,attributes:true,attributeFilter:['hidden','aria-current']});update();
 return {trigger,menu,select(){disclosure.select();update();},dispose(){changes.disconnect();disclosure.dispose();}};
}

export function experimentalBadge(){return el('span','badge maturity-experimental','Experimental');}
