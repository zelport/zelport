import {adornmentButton,button,el,experimentalBadge,icon,nextUIID,setText,withInfo} from './ui-dom.js';
import {disclosureMenu} from './ui-dialogs.js';

export function inputAdornment(input,{prefix='',suffix='',startActions=[],actions=[]}={}){const root=el('div','input-adornment');if(startActions.length){const start=el('div','input-adornment-actions');start.append(...startActions);root.append(start);}if(prefix)root.append(el('span','input-adornment-text',prefix));root.append(input);if(suffix)root.append(el('span','input-adornment-text',suffix));if(actions.length){const end=el('div','input-adornment-actions');end.append(...actions);root.append(end);}return root;}
export function adornedField(label,value='',type='text',adornments={}){const wrap=el('div','form-field'),caption=el('label',null,label),input=el('input');input.id=nextUIID('field');caption.htmlFor=input.id;input.type=type;input.value=value;const control=inputAdornment(input,adornments);wrap.append(caption,control);return {wrap,input,control};}
export function switchField(label,checked=false){const wrap=el('div','switch-field'),input=el('input'),caption=el('label',null,label);input.type='checkbox';input.checked=checked;input.id=nextUIID('switch');input.setAttribute('role','switch');caption.htmlFor=input.id;wrap.append(caption,input);return {wrap,input,caption};}
export function field(label,value='',type='text'){return type==='checkbox'?switchField(label,!!value):adornedField(label,value??'',type);}
export function numberStepperField(label,value='',{min=0,max=Number.MAX_SAFE_INTEGER,step=1}={}){let result;const change=delta=>{const current=Number(result.input.value),base=Number.isFinite(current)?current:min,next=Math.max(min,Math.min(max,base+(delta*step)));if(next!==current){result.input.value=String(next);result.input.dispatchEvent(new Event('input',{bubbles:true}));}result.input.focus();};const increment=adornmentButton(`Increase ${label}`,'caretUp',()=>change(1)),decrease=adornmentButton(`Decrease ${label}`,'caretDown',()=>change(-1)),controls=el('span','number-stepper-controls');controls.append(increment,decrease);result=adornedField(label,value,'number',{actions:[controls]});result.input.className='number-stepper-input';result.input.min=min;result.input.max=max;result.input.step=step;return {...result,increment,decrease};}
export function picker(label,options,value,{placeholder='Select…'}={}){const wrap=el('div','mapping-fields'),caption=el('label',null,label),root=el('div','mapping-dropdown'),trigger=el('button','mapping-trigger'),menu=el('div','mapping-menu');trigger.id=nextUIID('picker');caption.htmlFor=trigger.id;caption.id=`${trigger.id}-label`;trigger.setAttribute('aria-haspopup','listbox');menu.setAttribute('role','listbox');menu.setAttribute('aria-labelledby',caption.id);let current=value;const result={wrap,get value(){return current;},set disabled(disabled){trigger.disabled=disabled||!options.length;for(const child of menu.children)child.disabled=disabled;},onchange:null},disclosure=disclosureMenu(root,trigger,menu,{selected:n=>n.dataset.value===current});function select(next,notify=false){current=next;setText(trigger,options.find(option=>option.value===next)?.label||placeholder);disclosure.close();for(const child of menu.children)child.setAttribute('aria-selected',String(child.dataset.value===next));if(notify){result.onchange?.(next);root.dispatchEvent(new Event('change',{bubbles:true}));}}for(const option of options){const control=button(option.label,()=>{select(option.value,true);trigger.focus();},'mapping-option');control.dataset.value=option.value;control.setAttribute('role','option');menu.append(control);}trigger.disabled=!options.length;root.append(trigger,menu);wrap.append(caption,root);select(value);result.dispose=disclosure.dispose;return result;}
export function accordion(title,maturity){const root=el('details','settings-accordion'),summary=el('summary'),content=el('div','settings-accordion-content');summary.append(el('span','settings-accordion-title',title));if(maturity==='experimental')summary.append(experimentalBadge());root.append(summary,content);return {root,summary,content};}

// Keep validation beside its field; focus only after the caller restores controls.
export function formFeedback(form,error,fields=[]){
 const home=error.parentElement;
 let invalid=null,description=null;
 error.id ||= nextUIID('form-error');error.setAttribute('role','alert');error.tabIndex=-1;
 function clear(){
  if(invalid){invalid.removeAttribute('aria-invalid');if(description)invalid.setAttribute('aria-describedby',description);else invalid.removeAttribute('aria-describedby');invalid=null;}
  error.textContent='';if(home&&error.parentElement!==home)home.append(error);
 }
 function show(message,input){
  clear();if(!message)return;
  input ||= fields.find(field=>field.matches.test(message))?.input;
  error.textContent=message;
  if(input){
   invalid=input;description=input.getAttribute('aria-describedby');input.setAttribute('aria-invalid','true');input.setAttribute('aria-describedby',[description,error.id].filter(Boolean).join(' '));
   const wrap=input.closest?.('.form-field');if(wrap)wrap.append(error);
   for(let parent=input.parentElement;parent&&parent!==form;parent=parent.parentElement)if(parent.tagName==='DETAILS')parent.open=true;
  }
  const target=input&&!input.disabled?input:error;target.focus();target.scrollIntoView?.({block:'nearest',behavior:'instant'});
  error.scrollIntoView?.({block:'nearest',behavior:'instant'});
 }
 form.addEventListener('input',clear);
 return {show,clear};
}

// Service form building blocks shared by every source in the drawer.
// A choice row is one selectable line: the whole row picks it; trailing
// indicators stay separately focusable above the row's click area.
export function choiceRow({primary,secondary='',label,onSelect,trailing=[],disabled=false}){
 const row=el('div','choice-row'),main=button('',onSelect,'choice-main'),end=el('span','choice-trailing');
 main.append(el('span','choice-primary',primary));if(secondary){const detail=el('span','choice-secondary',secondary);detail.title=secondary;main.append(detail);}
 if(label)main.setAttribute('aria-label',label);main.setAttribute('aria-pressed','false');main.disabled=disabled;
 end.append(...trailing,icon('check',{className:'choice-check'}));row.append(main,end);row.dataset.selected='false';
 return {row,main,select(selected){row.dataset.selected=String(selected);main.setAttribute('aria-pressed',String(selected));}};
}
export function choiceList(label){const list=el('div','choice-list');list.setAttribute('role','list');list.setAttribute('aria-label',label);list.id=nextUIID('choices');return list;}
export function choiceGroup(name,status){const group=el('div','choice-group');group.append(el('span','choice-group-name',name));if(status)group.append(status);return group;}
export function choiceState(message,{warning=false,action}={}){const state=el('div',`choice-state${warning?' choice-state-warning':''}`);state.setAttribute('role','status');state.append(el('span','choice-state-message',message));if(action)state.append(action);return state;}
export function fieldHint(text,className=''){return el('p',`field-hint${className?` ${className}`:''}`,text);}
export function fieldRow(...wraps){const row=el('div','field-row');row.append(...wraps);return row;}
export function serviceOptions(){return el('div','service-options');}
// Option help is an info hint next to the switch label, not a sentence under it.
export function optionSwitch(label,checked=false,help=''){const row=el('div','option-row'),control=switchField(label,checked);if(help)withInfo(control,help);row.append(control.wrap);return {...control,row};}

// A finished action confirms itself at its trigger. A Save button swaps its label
// for a check ("✓" at the same width), a field action shows "✓ Copied" under its
// field. It leaves after 3 s or at the next edit. Errors stay.
export function confirmation(){
 const root=el('span','action-confirmation');root.setAttribute('role','status');root.setAttribute('aria-live','polite');root.hidden=true;let timer=null,target=null;
 const clear=()=>{if(timer)clearTimeout(timer);timer=null;root.replaceChildren();root.hidden=true;if(target)delete target.dataset.saved;};
 return {root,clear,
  // The button shows the check, the hidden live region still says the word.
  attach(button){target=button;root.className='action-confirmation action-confirmation-hidden';return this;},
  show(text='Saved'){if(timer)clearTimeout(timer);root.replaceChildren(...(target?[]:[icon('check',{className:'action-confirmation-glyph'})]),el('span',null,text));root.hidden=false;if(target)target.dataset.saved='true';timer=setTimeout(clear,3000);timer?.unref?.();}};
}
// While a submit runs, its button keeps its width and shows a spinner instead of the label.
export function busyButton(button,busy){button.setAttribute('aria-busy',String(!!busy));if(busy){button.dataset.busy='true';delete button.dataset.saved;}else delete button.dataset.busy;}
