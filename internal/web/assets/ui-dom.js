// Render keys describe displayed content, never configuration freshness.
const renderedViews=new WeakMap();
let fieldSequence=0;
export function nextUIID(prefix){return `${prefix}-${++fieldSequence}`;}
// A small info icon after a label; the explanation lives in its tooltip, never as text under the field.
export function infoHint(text,name='More information'){
 const root=el('span','info-hint'),trigger=el('button','info-hint-trigger'),tip=el('span','ui-tooltip info-hint-tooltip',text);trigger.type='button';trigger.setAttribute('aria-label',name);
 const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 16 16');svg.setAttribute('aria-hidden','true');svg.setAttribute('focusable','false');svg.setAttribute('class','info-hint-glyph');
 for(const [tag,attrs]of [['circle',{cx:8,cy:8,r:6.8}],['path',{d:'M8 7.3v3.9'}],['circle',{cx:8,cy:5,r:.7,class:'info-hint-dot'}]]){const node=document.createElementNS('http://www.w3.org/2000/svg',tag);for(const [k,v]of Object.entries(attrs))node.setAttribute(k,String(v));svg.append(node);}
 trigger.append(svg);const tooltip=attachTooltip(root,trigger,tip);trigger.addEventListener('click',event=>{event.preventDefault();tooltip.show();});root.append(trigger,tip);return {root,...tooltip};
}
// Puts an info hint right after a field's label (text fields and switches alike).
export function withInfo(control,text){
 const caption=control.caption||[...(control.wrap.children||[])].find(node=>node.tagName==='LABEL');if(!caption)return control;
 const hint=infoHint(text,`About ${caption.textContent}`);
 const row=el(control.wrap.className?.includes?.('switch-field')?'span':'div',control.wrap.className?.includes?.('switch-field')?'switch-label-row':'field-label-row'),rest=[...control.wrap.children].filter(node=>node!==caption);
 row.append(caption,hint.root);control.wrap.replaceChildren(row,...rest);
 control.hint=hint;return control;
}
export function setText(node,value){const text=String(value??'');if(node.textContent!==text)node.textContent=text;}
export function renderChanged(node,content,draw){const key=JSON.stringify(content);if(renderedViews.get(node)===key)return;draw();renderedViews.set(node,key);}
export function el(tag,className,text){const n=document.createElement(tag);if(className)n.className=className;if(text!==undefined)n.textContent=text;return n;}
export function button(text,fn,className='secondary'){const b=el('button',className,text);b.type='button';b.addEventListener('click',e=>{e.preventDefault();Promise.resolve(fn(e)).catch(()=>{});});return b;}

const iconPaths={
 edit:'M15 5l4 4M4 20l5-1L21 7a2.8 2.8 0 0 0-4-4L5 15l-1 5Z',
 shuffle:'M3 6h3c5 0 7 12 12 12h3M17 14l4 4-4 4M3 18h3c2 0 4-3 6-6s4-6 6-6h3M17 2l4 4-4 4',

 eye:'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Zm10-3a3 3 0 1 0 0 6 3 3 0 0 0 0-6',
 copy:'M9 9h11v12H9zM15 9V3H3v12h6',plus:'M12 5v14M5 12h14',caretDown:'M5 9l7 7 7-7z',caretUp:'M5 15l7-7 7 7z',
 arrowUp:'M12 20V4M6 10l6-6 6 6',rotate:'M20 7v5h-5M20 12a8 8 0 1 0-2 6',warning:'M12 3 2.5 20h19L12 3ZM12 9v5M12 17h.01',
 info:'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20ZM12 11v6M12 7h.01',localPort:'M4 4h16v6H4zM4 14h16v6H4zM8 7h.01M8 17h.01M12 7h5M12 17h5',
 container:'M12 3 20 7.5v9L12 21 4 16.5v-9L12 3ZM4 7.5 12 12l8-4.5M12 12v9',sandbox:'M12 7 17 10v5l-5 3-5-3v-5l5-3ZM7 10l5 3 5-3M12 13v5M4 7V4h3M17 4h3v3M20 17v3h-3M7 20H4v-3',
 laptop:'M4 5h16v10H4zM2 19h20',terminal:'M3 4h18v16H3zM7 9l3 3-3 3M12 15h5',globe:'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18ZM3 12h18M12 3c2.5 2.5 3.8 5.5 3.8 9s-1.3 6.5-3.8 9M12 3C9.5 5.5 8.2 8.5 8.2 12s1.3 6.5 3.8 9',
 devices:'M5 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5ZM19 3.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5ZM19 15.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5ZM7.3 11l9.4-4M7.3 13l9.4 4',
 check:'M5 12.5l4.5 4.5L19 7',chevronDown:'M6 9l6 6 6-6',chevronRight:'M9 6l6 6-6 6',external:'M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5',search:'M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14ZM20 20l-4-4',
 plugOff:'M8 2v4M16 2v4M7 6h10v3a5 5 0 0 1-10 0V6ZM12 14v7M3 3l18 18',experimental:'M9 3h6M10 3v7l-5 8a2 2 0 0 0 2 3h10a2 2 0 0 0 2-3l-5-8V3M8 15h8'
};
export function icon(name,{className='',title=''}={}){if(!iconPaths[name])throw new Error(`Unknown UI icon: ${name}`);const svg=document.createElementNS('http://www.w3.org/2000/svg','svg'),path=document.createElementNS('http://www.w3.org/2000/svg','path');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('aria-hidden','true');svg.setAttribute('focusable','false');if(className)svg.setAttribute('class',className);if(title){const caption=document.createElementNS('http://www.w3.org/2000/svg','title');caption.textContent=title;svg.append(caption);}path.setAttribute('d',iconPaths[name]);svg.append(path);return svg;}

const activeTooltipClosers=new Set(),tooltipBindings=new WeakMap();let tooltipViewportListeners=false;
function dismissTooltips(){for(const close of [...activeTooltipClosers])close();}
function ensureTooltipViewportListeners(){if(tooltipViewportListeners)return;window.addEventListener('scroll',dismissTooltips,true);window.addEventListener('resize',dismissTooltips);tooltipViewportListeners=true;}
function attachTooltip(root,trigger,tip){
 tip.id=nextUIID('tooltip');tip.setAttribute('role','tooltip');tip.setAttribute('popover','manual');tip.hidden=true;trigger.setAttribute('aria-describedby',tip.id);
 let hovered=false,focused=false,hoverTimer=null;
 const cancelPending=()=>{if(hoverTimer!==null){clearTimeout(hoverTimer);hoverTimer=null;}};
 function close(){cancelPending();activeTooltipClosers.delete(close);if(!tip.hidden)tip.hidePopover();tip.hidden=true;}
 function show(){
  cancelPending();if(root.hidden||!tip.textContent){close();return;}
  for(const dismiss of [...activeTooltipClosers])if(dismiss!==close)dismiss();
  activeTooltipClosers.add(close);ensureTooltipViewportListeners();tip.hidden=false;tip.showPopover();
  const anchor=trigger.getBoundingClientRect(),bounds=tip.getBoundingClientRect(),edge=12,gap=12;
  const center=anchor.left+anchor.width/2,left=Math.max(edge,Math.min(center-bounds.width/2,window.innerWidth-bounds.width-edge));
  const above=anchor.top-bounds.height-gap,below=anchor.bottom+gap;
  const placement=above>=edge||anchor.top>window.innerHeight-anchor.bottom?'top':'bottom';
  tip.dataset.placement=placement;
  tip.style.left=`${left}px`;tip.style.top=`${Math.max(edge,Math.min(placement==='top'?above:below,window.innerHeight-bounds.height-edge))}px`;
  tip.style.setProperty('--tooltip-arrow-x',`${Math.max(10,Math.min(center-left,bounds.width-10))}px`);
 }
 function schedule(){cancelPending();activeTooltipClosers.add(close);ensureTooltipViewportListeners();hoverTimer=setTimeout(()=>{hoverTimer=null;if(hovered)show();else activeTooltipClosers.delete(close);},250);}
 root.addEventListener('mouseenter',()=>{hovered=true;if(!focused)schedule();});
 root.addEventListener('mouseleave',()=>{hovered=false;cancelPending();if(!focused)close();});
 trigger.addEventListener('focus',()=>{focused=true;
  // Only keyboard focus opens the tooltip; focus moved by a dialog or script does not.
  if(typeof trigger.matches==='function'&&!trigger.matches(':focus-visible'))return;
  if(trigger.getAttribute('popovertarget')){queueMicrotask(()=>{const popover=document.getElementById(trigger.getAttribute('popovertarget'));if(focused&&!popover?.matches(':popover-open'))show();});}else show();});trigger.addEventListener('blur',()=>{focused=false;close();});
 root.addEventListener('keydown',event=>{if(event.key==='Escape'){if(!tip.hidden){event.preventDefault();event.stopPropagation();}close();}});
 const binding={tip,show,close,update(text){if(tip.textContent!==text){close();setText(tip,text);}}};tooltipBindings.set(trigger,binding);return binding;
}
export function setTooltip(trigger,description,{container=trigger.parentElement}={}){
 let tooltip=tooltipBindings.get(trigger);
 if(!tooltip){const tip=el('span','ui-tooltip');tooltip=attachTooltip(trigger,trigger,tip);container.append(tip);trigger.addEventListener('click',tooltip.close);}
 tooltip.update(description);return tooltip;
}
export function adornmentButton(label,name,fn,{tooltipClass=''}={}){
 let tooltip;const control=button('',event=>{tooltip.close();return fn(event);},'adornment-button'),tip=el('span',`ui-tooltip adornment-tooltip${tooltipClass?` ${tooltipClass}`:''}`,label);
 tooltip=attachTooltip(control,control,tip);control.setAttribute('aria-label',label);control.append(icon(name),tip);return control;
}
export function textTooltip(text,description,className=''){const root=el('span',`text-tooltip-trigger${className?` ${className}`:''}`,text),tip=el('span','ui-tooltip text-tooltip',description),tooltip=attachTooltip(root,root,tip);root.tabIndex=0;root.setAttribute('aria-label',description);root.append(tip);return {root,tip,...tooltip};}
export function warningIndicator(message='',label='Connection security warning'){const root=el('span','service-warning'),trigger=el('button','warning-trigger'),tip=el('span','ui-tooltip warning-tooltip');trigger.append(icon('warning',{className:'warning-glyph'}));trigger.type='button';trigger.setAttribute('aria-label',label);const tooltip=attachTooltip(root,trigger,tip);trigger.addEventListener('click',e=>{e.preventDefault();tooltip.show();});root.append(trigger,tip);const update=text=>{setText(tip,text);root.hidden=!text;if(!text)tooltip.close();};update(message);return {root,update};}
export function serviceTargetIndicator(description,domain){const root=el('span','service-target-indicator'),trigger=el('button','service-target-trigger'),tip=el('span','ui-tooltip service-target-tooltip',description);trigger.type='button';trigger.setAttribute('aria-label',`Details for ${domain}`);trigger.append(icon('info',{className:'service-target-glyph'}));const tooltip=attachTooltip(root,trigger,tip);trigger.addEventListener('click',e=>{e.preventDefault();tooltip.show();});root.append(trigger,tip);return {root,...tooltip};}
function detailIndicator(rootClass,triggerClass,tipClass,iconName,label,description){const root=el('span',rootClass),trigger=el('button',triggerClass),tip=el('span',`ui-tooltip ${tipClass}`);trigger.append(icon(iconName,{className:`${triggerClass.replace('-trigger','')}-glyph`}));trigger.type='button';trigger.setAttribute('aria-label',label);const tooltip=attachTooltip(root,trigger,tip);trigger.addEventListener('click',e=>{e.preventDefault();tooltip.show();});root.append(trigger,tip);setText(tip,description);root.hidden=!description;return {root,update:text=>{setText(tip,text);root.hidden=!text;if(!text)tooltip.close();},...tooltip};}
export function localPortIndicator(description=''){return detailIndicator('local-port-indicator','local-port-trigger','local-port-tooltip','localPort','Local port details',description);}
export function noListenerIndicator(description=''){return detailIndicator('no-listener-indicator','no-listener-trigger','no-listener-tooltip','plugOff','No listener detected',description);}
export function serviceURL(state,route){return state.remote_admin?state.remote_admin.service_links?.[route.id]||null:`http://${route.domain}:${state.port}`;}
export function moduleHeading(title){const heading=el('div','module-heading'),actions=el('div','module-actions');heading.id='module-heading';actions.id='module-actions';title.before(heading);heading.append(title,actions);return {heading,actions};}
export function emptyState({kind='no-results',message='No resources found.',action}={}){const card=el('section',`empty-state empty-state-${kind}`);card.setAttribute('role',kind==='unavailable'?'alert':'status');card.append(el('p','empty-state-message',message));if(action)card.append(button(action.label,action.run,action.className||'primary'));return card;}
export function statusTone(value){const state=String(value||'').toLowerCase();if(['ready','running','listening','connected','available'].includes(state))return 'ok';if(['error','failed','unavailable'].includes(state))return 'error';if(['stale','waiting','connecting','pending'].includes(state))return 'pending';return 'neutral';}
export function statusBadge(value){const badge=el('span','badge status-badge',value);badge.dataset.tone=statusTone(value);return badge;}
export function experimentalBadge(){return el('span','badge maturity-experimental','Experimental');}
// One-line service status. Detail is a tooltip on wide screens and a line on phones.
export function serviceStatus(){
 const root=el('span','service-status'),mark=el('span','service-status-mark'),label=el('span','service-status-label'),detail=el('span','service-status-detail'),tip=el('span','ui-tooltip status-tooltip');
 const tooltip=attachTooltip(root,label,tip);root.append(mark,label,detail,tip);
 function update({tone='neutral',label:text='',detail:more=''}={}){
  // Colour lives only in the dot; the label reads the same for every tone.
  if(root.dataset.tone!==tone)root.dataset.tone=tone;
  setText(label,text);setText(detail,more);setText(tip,more);detail.hidden=!more;
  if(more)label.tabIndex=0;else{label.removeAttribute('tabindex');tooltip.close();}
 }
 return {root,update};
}
