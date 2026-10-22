export function createClient({onState=()=>{},onError=()=>{},owner=()=>''}={}){
 let current,pending,busy=false,disposed=false;
 async function api(path,options={}){
  if(!path.startsWith('/api/')||path.includes('://'))throw Error('Invalid API path');
  const method=options.method||'GET',headers={...options.headers};
  if(!['GET','HEAD'].includes(method)){headers['X-CSRF-Token']=current?.csrf;headers['Content-Type']='application/json';}
  const response=await fetch(path,{...options,headers}),text=await response.text();let data;
  try{data=JSON.parse(text);}catch{data={error:text};}
  if(!response.ok){const error=Error(data.error||`Request failed (${response.status})`);error.status=response.status;throw error;}
  return data;
 }
 async function refresh(force=false){if(disposed)return current;if(pending){if(!force)return pending;await pending.catch(()=>{});}pending=(async()=>{const next=await api('/api/state');if(!disposed){current=next;onState(next);}return next;})().finally(()=>pending=null);return pending;}
 async function mutate(work,{reportError=true}={}){const operationOwner=owner();if(busy)throw Error('Another operation is in progress');busy=true;onError('',operationOwner);document.body.setAttribute('aria-busy','true');try{const result=await work();await refresh(true);return result;}catch(error){if(reportError)onError(error.message,operationOwner);await refresh(true).catch(()=>{});throw error;}finally{busy=false;document.body.removeAttribute('aria-busy');}}
 return {api,refresh,mutate,state:()=>current,dispose(){disposed=true;}};
}
