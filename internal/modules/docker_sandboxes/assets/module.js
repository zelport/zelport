export function sandboxStatus(state){
 const data=state.data||{},cli=data.cli||{};
 const status=cli.status==='not_found'?'CLI not found':data.error?'Discovery failed':data.stale?(data.updated_at&&!data.updated_at.startsWith('0001-')?'Partial discovery failed':'Checking'):'Available';
 return {version:cli.version||(cli.status==='found'?'Version unavailable':''),path:cli.path||'',status,error:[cli.error,data.error,...Object.values(data.port_errors||{})].filter(Boolean).join('\n')};
}
export function headerStatus(state){
 if(!state?.data)return null;
 const info=sandboxStatus(state);
 return {status:info.status,tone:info.status==='Available'?'ok':info.status==='Checking'?'neutral':'error',action:'Open module',rows:[
  {label:'CLI',value:'sbx'},{label:'Version',value:info.version||'Unavailable'},
  ...(info.path?[{label:'Path',value:info.path}]:[]),
  ...(info.error?[{label:'Error',value:info.error}]:[])
 ]};
}
export function mount(ctx){const {el,field,picker,serviceModule}=ctx.ui;
 const list=el('div');ctx.panel.append(list);
const action=(label,path,id)=>({label,run:()=>ctx.mutate(()=>ctx.api(`/api/modules/${ctx.moduleId}/${path}`,{method:'POST',body:JSON.stringify({id,revision:ctx.ui.state().revision})}))});const view=serviceModule({...ctx,panel:list},{refresh:true,routeActions:r=>r.summary.state==='unavailable'?[action('Start sandbox','start',r.id)]:[],removeActions:r=>[action('Remove proxy and port mapping','remove-mapping',r.id)],groupActions:g=>[action('Open terminal','terminal',g.id)],groups:s=>(s.data?.sandboxes||[]).map(b=>({...b,key:b.id})),matches:(r,g)=>r.editor.sandbox_id===g.id,fields(s,g,state){const box=g||(state.data?.sandboxes||[]).find(b=>b.id===s.sandbox_id)||{id:s.sandbox_id,name:s.sandbox_name,ports:[]};const ports=(box.ports||[]).filter(p=>['tcp','tcp4','tcp6'].includes(p.protocol)&&['127.0.0.1','::1'].includes(p.host_ip));const seen=new Set(),unique=ports.filter(p=>{const key=`${p.sandbox_port}:${p.host_port}`;if(seen.has(key))return false;seen.add(key);return true;});let selected=unique.findIndex(p=>s.selected&&p.host_port===s.selected.host_port&&p.sandbox_port===s.selected.sandbox_port);if(selected<0)selected=unique.findIndex(p=>p.sandbox_port===s.sandbox_port);const options=unique.map((p,i)=>({value:String(i),label:`Existing mapping: Sandbox port ${p.sandbox_port} → Aux port ${p.host_port}`}));options.push({value:'new',label:'Create new mapping…'});const mapping=picker('Port mapping',options,selected>=0?String(selected):s.sandbox_port?'new':''),port=field('Sandbox port',s.sandbox_port||3000,'number');port.input.min=1;port.input.max=65535;const hint=el('p','small muted','An Aux port will be assigned automatically.');const update=()=>{port.wrap.hidden=mapping.value!=='new';port.input.required=mapping.value==='new';hint.hidden=port.wrap.hidden;};mapping.onchange=update;update();const element=el('div');element.append(mapping.wrap,port.wrap,hint);return {element,read(){if(!mapping.value)throw Error('Choose a port mapping');const p=mapping.value==='new'?null:unique[Number(mapping.value)];return {sandbox_id:box.id,sandbox_name:box.name,sandbox_port:p?p.sandbox_port:Number(port.input.value),selected:p};}};}});return {...view,render(state){
 const data={...state.data,error:undefined};view.render({...state,data});list.hidden=!!state.data?.error;
 }};
}
