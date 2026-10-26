import {el,button,nextUIID,setTooltip} from './ui-dom.js';
import {formFeedback} from './ui-forms.js';
import {showDialog} from './ui-dialogs.js';

// Contents are requested only for this dialog and cleared when it closes.
export function fileEditor(ctx,{endpoint,title,trigger,onSaved=()=>{},onClose=()=>{}}){
 const dialog=el('dialog','file-editor-dialog'),header=el('div','dialog-header'),heading=el('h2',null,title),close=button('×',()=>requestClose(),'icon-button');
 heading.id=nextUIID('file-editor-title');dialog.setAttribute('aria-labelledby',heading.id);close.setAttribute('aria-label','Close editor');setTooltip(close,'Close',{container:close});header.append(heading,close);
 const target=el('p','file-editor-target small muted'),path=el('p','file-editor-path'),label=el('label','file-editor-label','File contents'),area=el('textarea','file-editor-text'),error=el('p','route-error'),status=el('p','small muted','Loading…'),actions=el('div','dialog-actions'),confirm=el('div','file-editor-discard');
 const remote=ctx.ui.state().remote_admin;target.textContent=remote?(remote.hostname||remote.device_id||'Remote Zelport'):'';target.hidden=!remote;
 area.id=nextUIID('file-editor-content');label.htmlFor=area.id;area.spellcheck=false;area.autocomplete='off';area.setAttribute('autocapitalize','off');area.setAttribute('wrap','off');area.setAttribute('aria-describedby',nextUIID('file-editor-path'));path.id=area.getAttribute('aria-describedby');
 error.setAttribute('role','alert');status.setAttribute('role','status');
 let documentValue=null,baseline='',loading=true,saving=false,closed=false,conflict=false,discardAction='close',lineEnding='\n';
 const cancel=button('Cancel',()=>requestClose()),save=button('Save',saveFile,'primary'),reload=button('Reload file',()=>requestClose('reload'));
 const discard=button('Discard',()=>{confirm.hidden=true;if(discardAction==='reload')load();else dialog.close();},'danger'),keep=button('Keep editing',()=>{confirm.hidden=true;area.focus();});
 confirm.append(el('span',null,'Discard unsaved changes?'),keep,discard);confirm.hidden=true;reload.hidden=true;actions.append(reload,cancel,save);
 dialog.append(header,target,path,label,area,error,status,confirm,actions);
 const validation=formFeedback(dialog,error);
 const dirty=()=>documentValue!==null&&area.value!==baseline;
 function update(){area.disabled=loading||saving||!documentValue;save.disabled=loading||saving||!documentValue||!dirty()||conflict;cancel.disabled=close.disabled=saving;reload.disabled=saving||loading;discard.disabled=keep.disabled=saving;dialog.setAttribute('aria-busy',String(loading||saving));}
 function requestClose(action='close'){
  if(closed||saving)return;
  if(dirty()){discardAction=action;confirm.hidden=false;keep.focus();return;}
  if(action==='reload')load();else dialog.close();
 }
 async function load(){
  if(closed||saving)return;loading=true;validation.clear();status.textContent='Loading…';update();
  try{
   const value=await ctx.api(endpoint,{method:'GET'});if(closed)return;
   documentValue=value;area.value=value.content;baseline=area.value;lineEnding=/\r\n/.test(value.content)&&!/(^|[^\r])\n|\r(?!\n)/.test(value.content)?'\r\n':'\n';path.textContent=value.path;conflict=false;reload.hidden=true;confirm.hidden=true;status.textContent='';area.scrollTop=area.scrollLeft=0;
  }catch(cause){if(!closed){error.textContent=cause.message;status.textContent='';reload.hidden=false;}}
  finally{loading=false;if(!closed){update();if(error.textContent)validation.show(error.textContent);else area.focus();}}
 }
 async function saveFile(){
  if(closed||save.disabled)return;saving=true;validation.clear();status.textContent='Saving…';confirm.hidden=true;update();
  try{
   await ctx.api(endpoint,{method:'PUT',body:JSON.stringify({revision:documentValue.revision,version:documentValue.version,content:lineEnding==='\r\n'?area.value.replace(/\n/g,'\r\n'):area.value})});
   if(closed)return;baseline=area.value;saving=false;dialog.close();await onSaved();
  }catch(cause){
   if(!closed){error.textContent=cause.status===409?'The file changed. Copy your draft or reload the file.':cause.message;conflict=cause.status===409;reload.hidden=!conflict;status.textContent='';}
  }finally{saving=false;if(!closed){update();if(error.textContent)validation.show(error.textContent);}}
 }
 const beforeUnload=event=>{if(dirty()||saving){event.preventDefault();event.returnValue='';}};
 area.addEventListener('input',()=>{validation.clear();update();});
 area.addEventListener('keydown',event=>{if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='s'){event.preventDefault();saveFile();}});
 dialog.addEventListener('cancel',event=>{event.preventDefault();requestClose();});
 window.addEventListener('beforeunload',beforeUnload);
 showDialog(dialog,{signal:ctx.signal,initialFocus:cancel,onClose:()=>{closed=true;area.value='';baseline='';documentValue=null;window.removeEventListener('beforeunload',beforeUnload);onClose();trigger?.focus();}});
 update();load();
 return {dispose(){if(!closed)dialog.close();}};
}
