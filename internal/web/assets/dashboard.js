// The service table: Dashboard lists every service; a source page lists only its own
// services (moduleId) without the Source column. One row each, with status and actions.
// Status is the service state only; target security properties come from routeWarning(route).
export const sourceChoices=[
 {id:'local_ports',description:'This computer',icon:'laptop'},
 {id:'ports_over_ssh',description:'A server',icon:'terminal'},
 {id:'remote_http',description:'Any URL',icon:'globe'},
 {id:'ports_over_relay',description:'A linked Zelport',icon:'devices'}
];
// Sources are named by connection; the Source column and menus use these short names.
const sourceLabels={local_ports:'Local',ports_over_ssh:'SSH',remote_http:'HTTP',ports_over_relay:'Relay'};
export function sourceLabel(id,fallback){return sourceLabels[id]||fallback||id;}
// Tones: ok (green), warning (amber: target offers nothing now), error (red: connection failed), neutral (grey outline: not known).
export function defaultRouteStatus(route){
 const summary=route.summary||{};
 if(summary.state==='unavailable')return {tone:'warning',label:'Unavailable',detail:summary.detail||''};
 if(summary.state==='ready')return {tone:'ok',label:'Available',detail:summary.detail||''};
 if(summary.detail)return {tone:'warning',label:'Warning',detail:summary.detail};
 return {tone:'neutral',label:'Not checked'};
}
const sortable=new Set(['Address','Source','Status']);
// "Problems first" when sorting by Status.
const statusRank={error:0,warning:1,neutral:2,ok:3};
// A query without "." or ":" matches the service name only; with one it matches the whole address.
export function addressMatches(query,domain,address){
 const q=String(query||'').trim().toLowerCase();if(!q)return true;
 if(/[.:]/.test(q))return String(address||'').toLowerCase().includes(q);
 return String(domain||'').toLowerCase().replace(/\.localhost$/,'').includes(q);
}
function storedSort(key){try{const value=JSON.parse(localStorage.getItem(key)||'null');if(value&&sortable.has(value.column)&&(value.direction===1||value.direction===-1))return value;}catch{}return {column:'Address',direction:1};}
function storeSort(key,value){try{localStorage.setItem(key,JSON.stringify(value));}catch{}}
// The Zelport whose browser administers a linked one, by name. "This computer" would be ambiguous there.
export function remoteConsumerName(state){return state?.remote_admin?.consumer_hostname||'Your Zelport';}
export function createDashboard({root,modules,client,navigate,ui,moduleId=null,returnTo='dashboard',tools=null,onRescan=null}){
 // While administering a linked Zelport its addresses are plain text (*.localhost always opens this computer);
 // A column named after the administering Zelport links to its local imports of those services.
 const remote=!!client.state()?.remote_admin,localColumn=remoteConsumerName(client.state());
 const rows=new Map(),columns=[...(moduleId?['Address','Target','Status']:['Address','Source','Target','Status']),...(remote?[localColumn]:[]),'Actions'];let table,body,empty,noMatches;
 const sortKey=`zelport.sort.${moduleId||'dashboard'}`,headers=new Map();let sort=storedSort(sortKey),query='',order='';
 const enabled=module=>module?.descriptor.ui_placement==='settings'||client.state()?.module_availability?.[module?.descriptor.id]?.enabled!==false;
 // Search and Rescan ports live in the page heading next to Add service.
 let search,input,clear,rescan;
 if(tools){
  search=ui.el('label','table-search');input=ui.el('input');input.type='search';input.placeholder='Search';input.autocomplete='off';input.spellcheck=false;input.setAttribute('aria-label','Search services by address');
  clear=ui.el('button','table-search-clear','×');clear.type='button';clear.setAttribute('aria-label','Clear search');clear.hidden=true;
  search.append(ui.icon('search',{className:'table-search-icon'}),input,clear);
  const setQuery=value=>{query=value;input.value=value;clear.hidden=!value;applyView();};
  input.addEventListener('input',()=>setQuery(input.value));
  input.addEventListener('keydown',event=>{if(event.key==='Escape'&&input.value){event.preventDefault();setQuery('');}});
  clear.addEventListener('click',()=>{setQuery('');input.focus();});
  tools.append(search);
  if(onRescan){
   rescan=ui.refreshControl('Rescan ports',async()=>{if(rescan.disabled)return;rescan.disabled=true;try{await onRescan();}finally{rescan.disabled=false;}});
   rescan.classList.add('rescan-ports');tools.append(rescan);
  }
 }
 // Add service is one drawer; choosing another source swaps its content in place.
 function openAddService(trigger,{sourceId=moduleId,domain,container}={}){
  const sources=sourceChoices.map(choice=>{const module=modules.get(choice.id);return {...choice,...(choice.id==='local_ports'&&client.state()?.remote_admin?{description:client.state().remote_admin.hostname||choice.description}:{}),label:module?.descriptor.name||choice.id,available:!!module?.instance&&enabled(module)};}).filter(choice=>choice.available);
  const selected=sources.find(choice=>choice.id===sourceId)||sources[0];
  if(!selected)return null;
  return modules.get(selected.id).instance.edit(null,null,{sources,sourceId:selected.id,domain,trigger,container,
   onSource(id,draft){openAddService(trigger,{sourceId:id,...draft});},
   onSaved(){navigate(returnTo);}});
 }
 function addressText(state,route,address){if(state.remote_admin&&address){try{return new URL(address).host;}catch{}}return state.port===80?route.domain:`${route.domain}:${state.port}`;}
 function headerCell(name){
  const cell=ui.el('th');cell.scope='col';
  if(name==='Actions'){cell.className='visually-hidden-label';cell.textContent=name;return cell;}
  if(remote&&name===localColumn)cell.className='col-local';
  if(!sortable.has(name)){cell.textContent=name;return cell;}
  const control=ui.el('button','sort-button',name);control.type='button';
  const arrow=ui.el('span','sort-arrow');arrow.setAttribute('aria-hidden','true');control.append(arrow);
  control.addEventListener('click',()=>{sort=sort.column===name?{column:name,direction:-sort.direction}:{column:name,direction:1};storeSort(sortKey,sort);applyView();});
  cell.append(control);headers.set(name,{cell,arrow});return cell;
 }
 function compare(a,b){
  const byName=a.route.domain.localeCompare(b.route.domain);
  if(sort.column==='Source')return sort.direction*(a.source.textContent.localeCompare(b.source.textContent)||byName);
  if(sort.column==='Status')return sort.direction*((statusRank[a.tone]??2)-(statusRank[b.tone]??2)||byName);
  return sort.direction*byName;
 }
 // Sort, then filter by the query; rows are moved only when the order changes.
 function applyView(){
  if(!body)return;
  for(const [name,{cell,arrow}]of headers){const on=sort.column===name;cell.setAttribute('aria-sort',on?(sort.direction===1?'ascending':'descending'):'none');arrow.textContent=on?(sort.direction===1?'▲':'▼'):'';}
  const items=[...rows.values()].sort(compare),next=items.map(item=>item.route.id).join(' ');
  if(next!==order){for(const item of items)body.append(item.row);order=next;}
  let shown=0;for(const item of items){const match=addressMatches(query,item.route.domain,item.addressText);item.row.hidden=!match;if(match)shown++;}
  noMatches.hidden=!(items.length&&!shown);if(!noMatches.hidden)body.append(noMatches);
 }
 function render(state=client.state()){
  const routes=(moduleId?[state.states[moduleId]||{}]:Object.values(state.states)).flatMap(module=>module.routes||[]);
  if(!table){
   const wrap=ui.el('div','dashboard-table-wrap');table=ui.el('table',`dashboard-table data-table${moduleId?' source-table':''}`);
   const head=ui.el('tr'),thead=ui.el('thead');body=ui.el('tbody');
   for(const name of columns)head.append(headerCell(name));
   noMatches=ui.el('tr','no-matches-row');const cell=ui.el('td',null,'No matches');cell.colSpan=columns.length;noMatches.append(cell);noMatches.hidden=true;
   thead.append(head);table.append(thead,body);wrap.append(table);empty=ui.el('div','empty dashboard-empty');root.append(wrap,empty);
  }
  table.parentElement.hidden=!routes.length;empty.hidden=!!routes.length;
  if(search)search.hidden=!routes.length;
  if(rescan){const local=modules.get('local_ports');rescan.hidden=!(local&&enabled(local)&&routes.some(route=>route.module==='local_ports'));}
  // A source can name what is missing before it has services (Relay: "Configure relay first").
  const emptyLabel=moduleId?modules.get(moduleId)?.instance?.emptyLabel?.():'';
  if(!routes.length)ui.renderChanged(empty,{modules:!!state.modules.length,emptyLabel},()=>{
   empty.replaceChildren(ui.el('p',null,state.modules.length?emptyLabel||'No services':'This build contains only the core.'));
  });
  const present=new Set(routes.map(route=>route.id));
  for(const [id,item] of rows)if(!present.has(id)){item.menu.dispose();item.row.remove();rows.delete(id);}
  for(const route of routes){
   let item=rows.get(route.id);
   if(!item){
    const row=ui.el('tr'),local=ui.el('td','service-local'),domain=ui.el('td','service-address'),source=ui.el('td','service-source'),target=ui.el('td','service-target'),status=ui.el('td','service-status-cell'),action=ui.el('td','service-actions-cell');
    for(const [cell,label]of [[domain,'Address'],[source,'Source'],[target,'Target'],[status,'Status'],[local,localColumn],[action,'Actions']])cell.dataset.label=label;
    const indicator=ui.serviceStatus(),actionRoot=ui.el('div','resource-actions dashboard-actions'),trigger=ui.el('button','resource-menu-trigger','⋮'),menu=ui.el('div','resource-menu');
    trigger.setAttribute('aria-haspopup','menu');menu.setAttribute('role','menu');actionRoot.append(trigger,menu);action.append(actionRoot);status.append(indicator.root);
    // A security property of the target (for example unencrypted) is a warning after the target text, so targets stay aligned; Status stays the service state.
    const warning=ui.warningIndicator('',`Security warning for ${route.domain}`),targetLine=ui.el('span','service-target-line'),targetText=ui.el('span','service-target-text'),targetDetail=ui.el('span','service-target-detail');targetLine.append(targetText,warning.root);target.append(targetLine,targetDetail);
    item={row,local,domain,source,target,targetText,targetDetail,warning,indicator,trigger,route,menu:ui.disclosureMenu(actionRoot,trigger,menu)};rows.set(route.id,item);
    const open=ui.el('a','resource-menu-item','Open');open.setAttribute('role','menuitem');open.target='_blank';open.rel='noopener';open.addEventListener('click',()=>item.menu.close(true));
    const edit=ui.button('Edit',()=>{item.menu.close(true);modules.get(item.route.module)?.instance?.edit(item.route);},'resource-menu-item');
    const remove=ui.button('Remove',()=>{item.menu.close(true);modules.get(item.route.module)?.instance?.remove?.(item.route);},'resource-menu-item service-menu-remove');
    for(const control of [edit,remove])control.setAttribute('role','menuitem');
    menu.append(open,edit,remove);item.open=open;item.remove=remove;
    // applyView places the row in sort order.
    row.append(...(moduleId?[domain,target,status]:[domain,source,target,status]),...(remote?[local]:[]),action);
   }
   item.route=route;item.trigger.setAttribute('aria-label',`Actions for ${route.domain}`);
   const address=ui.serviceURL(state,route),linked=!!address,instance=modules.get(route.module)?.instance,addressLinked=linked&&!remote;
   if(!item.link||item.linked!==addressLinked){item.link=ui.el(addressLinked?'a':'span');item.linked=addressLinked;item.domain.replaceChildren(item.link);if(addressLinked){item.link.target='_blank';item.link.rel='noopener';}}
   item.addressText=remote?addressText(state,route,null):addressText(state,route,address);ui.setText(item.link,item.addressText);
   if(addressLinked&&item.link.href!==address)item.link.href=address;if(linked&&item.address!==address)item.open.href=address;item.address=address;item.open.hidden=!linked;
   if(remote)ui.renderChanged(item.local,[address],()=>{item.local.replaceChildren();if(!address)return;const anchor=ui.el('a','service-local-link');anchor.href=address;anchor.target='_blank';anchor.rel='noopener';let host=address;try{host=new URL(address).hostname.replace(/\.localhost$/,'');}catch{}anchor.append(ui.el('span',null,host),ui.icon('external',{className:'service-local-icon'}));anchor.setAttribute('aria-label',`${host} on this computer, opens in a new tab`);item.local.append(anchor);});
   item.remove.hidden=!instance?.remove;
   const label=sourceLabel(route.module,modules.get(route.module)?.descriptor.name);ui.setText(item.source,label);item.targetText.parentElement.dataset.source=label;
   ui.setText(item.targetText,instance?.routeTarget?.(route)||route.summary.target||'');const warning=instance?.routeWarning?.(route)||'';item.warning.update(warning);ui.setText(item.targetDetail,warning);item.targetDetail.hidden=!warning;
   // An unreachable linked Zelport keeps its last list, but no status is current any more.
   const health=client.health?.(),lastSeen=health?.since?` Last checked ${health.since.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'})}.`:'';
   const status=health?.unreachable?{tone:'neutral',label:'Unknown',detail:`No answer from this Zelport.${lastSeen}`}:{...(instance?.routeStatus?.(route)||defaultRouteStatus(route))};
   item.tone=status.tone;item.indicator.update(status);
  }
  applyView();
 }
 return {render,openAddService,dispose(){for(const item of rows.values())item.menu.dispose();rows.clear();}};
}
