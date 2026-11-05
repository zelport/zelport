export function createClient({onState=()=>{},onError=()=>{},owner=()=>''}={}){
 let current,pending,busy=false,disposed=false;
 // Status errors carry a title and a reason ("build-server is unreachable" + "Relay is disconnected.").
 function errorMessage(data,status){
  if(data.status&&data.error&&data.detail)return `${data.error}. ${data.detail}`;
  return data.error||`Request failed (${status})`;
 }
 async function api(path,options={}){
  if(!path.startsWith('/api/')||path.includes('://'))throw Error('Invalid API path');
  const method=options.method||'GET',headers={Accept:'application/json',...options.headers};
  if(!['GET','HEAD'].includes(method)){headers['X-CSRF-Token']=current?.csrf;headers['Content-Type']='application/json';}
  const response=await fetch(path,{...options,headers}),text=await response.text();let data;
  // A body that is not JSON (an HTML status page from a proxy or tunnel) is never shown as the message.
  try{data=JSON.parse(text);}catch{data={};}
  if(!response.ok){const error=Error(errorMessage(data,response.status));error.status=response.status;throw error;}
  return data;
 }
 async function refresh(force=false){if(disposed)return current;if(pending){if(!force)return pending;await pending.catch(()=>{});}pending=(async()=>{const next=await api('/api/state');if(!disposed){current=next;onState(next);}return next;})().finally(()=>pending=null);return pending;}
 async function mutate(work,{reportError=true}={}){const operationOwner=owner();if(busy)throw Error('Another operation is in progress');busy=true;onError('',operationOwner);document.body.setAttribute('aria-busy','true');try{const result=await work();await refresh(true);return result;}catch(error){if(reportError)onError(error.message,operationOwner);await refresh(true).catch(()=>{});throw error;}finally{busy=false;document.body.removeAttribute('aria-busy');}}
 return {api,refresh,mutate,state:()=>current,dispose(){disposed=true;}};
}
