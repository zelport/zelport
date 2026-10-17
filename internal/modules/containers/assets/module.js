const engines=[['docker','Docker'],['podman','Podman'],['nerdctl','nerdctl']];
export function engineStatuses(state){
 return engines.map(([id,name])=>{
  const e=(state.data?.engines||[]).find(e=>e.engine===id)||{};
  const used=(state.routes||[]).some(r=>r.editor?.container?.engine===id);
  const cli=e.cli_status==='found'?'Found':e.cli_status==='not_found'?'Not found':'Checking';
  const runtime=({available:'Available',unavailable:'Unavailable',permission_denied:'Permission denied'})[e.runtime_status]||'Not checked';
  const problem=!!e.error||e.stale;
  const tone=problem?(used?'error':cli==='Not found'?'neutral':'pending'):runtime==='Available'?'ok':'neutral';
  return {id,name,cli,runtime,tone,path:e.cli_path||'',connection:e.label||'',endpoint:e.endpoint||'',namespace:e.namespace||'',error:e.error||'',experimental:id==='nerdctl'};
 });
}
export function headerStatus(state){
 if(!state?.data)return null;
 const engines=engineStatuses(state),problem=engines.some(e=>e.tone==='error'||e.tone==='pending'),available=engines.some(e=>e.runtime==='Available');
 return {status:problem?'Needs attention':available?'Available':'Unavailable',tone:problem?'pending':available?'ok':'neutral',action:'Open module',
  sections:engines.map(e=>({title:e.name+(e.experimental?' · Experimental':''),rows:[
   {label:'CLI',value:e.cli},{label:'Runtime',value:e.runtime},
   ...(e.path?[{label:'Path',value:e.path}]:[]),
   ...(e.connection?[{label:'Connection',value:e.connection}]:[]),
   ...(e.namespace?[{label:'Namespace',value:e.namespace}]:[]),
   ...(e.runtime==='Permission denied'?[{label:'Access',value:`The user running Zelport lacks permission to access ${e.name}. Check the selected connection and its permissions.`}]:[])
  ],diagnostic:e.cli==='Found'?e.error:'',
  help:e.id==='docker'&&e.runtime==='Permission denied'&&/^unix:\/\/\/(?:var\/)?run\/docker\.sock$/.test(e.endpoint)?{
   text:'For rootful Docker on Linux, run this on the Docker host as the user running Zelport:',
   command:'sudo usermod -aG docker "$USER"',
   after:'Sign out completely, then sign in again and restart Zelport. Existing processes keep their previous group membership.',
   note:'With rootful Docker, membership in the docker group grants root-level control of the host. Rootless Docker is an alternative.',
   linkLabel:'Docker documentation',
   url:'https://docs.docker.com/engine/install/linux-postinstall/'
  }:undefined}))};
}
export function containerGroups(state){
 const result=[],seen=new Map();
 for(const e of state.data?.engines||[]){
  for(const c of e.containers||[]){
   const key=`${e.engine}:${e.connection}:${c.id}`,alias={engine:e.engine,connection:e.connection};
   // Only a proven shared local socket and full container ID identify duplicates.
   const shared=!e.stale&&e.endpoint?`${e.endpoint}:${c.id}`:key;
   const previous=seen.get(shared);
   if(previous){previous.aliases.push(alias);previous.source+=' / '+(engines.find(([id])=>id===e.engine)?.[1]||e.engine);continue;}
   const group={...c,key,engine:e.engine,connection:e.connection,aliases:[alias],source:engines.find(([id])=>id===e.engine)?.[1]||e.engine};
   if(e.namespace)group.source+=` · ${e.namespace}`;
   result.push(group);seen.set(shared,group);
  }
 }
 return result;
}
export function mount(ctx){
 const {el,picker,serviceModule}=ctx.ui;
 const list=el('div');ctx.panel.append(list);
 const view=serviceModule({...ctx,panel:list},{refresh:true,groups:containerGroups,
 matches:(r,g)=>g.aliases.some(a=>r.editor?.container?.engine===a.engine&&r.editor?.container?.connection===a.connection)&&r.editor?.container?.id===g.id,
fields(s,g,state){const saved=s.container;let box=g;if(!box&&saved){for(const e of state.data?.engines||[])if(e.engine===saved.engine&&e.connection===saved.connection){const c=e.containers.find(c=>c.id===saved.id);if(c)box={...c,engine:e.engine,connection:e.connection};}}box=box||{id:saved?.id,name:saved?.name,engine:saved?.engine,connection:saved?.connection,ports:saved?[saved.selected]:[]};const ports=(box.ports||[]).filter(p=>p.protocol==='tcp'&&['127.0.0.1','::1'].includes(p.host_ip));const selected=ports.findIndex(p=>saved&&p.host_port===saved.selected.host_port&&p.container_port===saved.port);const mapping=picker('Existing port mapping',ports.map((p,i)=>({value:String(i),label:`Container port ${p.container_port} → Aux port ${p.host_port}`})),saved&&selected>=0?String(selected):'');const element=el('div');element.append(mapping.wrap);if(!ports.length)element.append(el('p','route-error','No usable published ports. Publish a port using the container engine first.'));return {element,read(){if(!mapping.value)throw Error('Select an existing published port');const p=ports[Number(mapping.value)];if(!p)throw Error('Select an existing published port');return {container:{engine:box.engine,connection:box.connection,id:box.id,name:box.name,port:p.container_port,selected:p}};}};} });
 return view;
}
