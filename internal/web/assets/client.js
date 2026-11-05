export function createClient({onState=()=>{},onError=()=>{},onHealth=()=>{},owner=()=>''}={}){
 let current,pending,busy=false,disposed=false,lastOk=null,health={unreachable:false,reason:'',since:null};
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
  if(!response.ok){const error=Error(errorMessage(data,response.status));error.status=response.status;error.detail=data.detail||'';throw error;}
  return data;
 }
 async function refresh(force=false){if(disposed)return current;if(pending){if(!force)return pending;await pending.catch(()=>{});}pending=(async()=>{let next;try{next=await api('/api/state');}catch(error){noteFailure(error);throw error;}lastOk=new Date();if(!disposed){current=next;if(health.unreachable){health={unreachable:false,reason:'',since:null};onHealth(health);}onState(next);}return next;})().finally(()=>pending=null);return pending;}
 // While administering a linked Zelport, a failed state read (tunnel, relay or the Zelport itself) marks it unreachable.
 // The last good state stays on screen until it answers again.
 function noteFailure(error){if(disposed||!current?.remote_admin||(error.status&&error.status<500))return;const reason=error.detail||'';if(health.unreachable&&health.reason===reason)return;health={unreachable:true,reason,since:lastOk};onHealth(health);}
 async function mutate(work,{reportError=true}={}){const operationOwner=owner();if(busy)throw Error('Another operation is in progress');busy=true;onError('',operationOwner);document.body.setAttribute('aria-busy','true');try{const result=await work();await refresh(true);return result;}catch(error){if(reportError)onError(error.message,operationOwner);await refresh(true).catch(()=>{});throw error;}finally{busy=false;document.body.removeAttribute('aria-busy');}}
 return {api,refresh,mutate,state:()=>current,health:()=>health,dispose(){disposed=true;}};
}
