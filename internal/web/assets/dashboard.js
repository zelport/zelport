// Services is the only list of services: one row each, with its status and actions.
export const sourceChoices=[
 {id:'local_ports',description:'A port on this computer'},
 {id:'ports_over_ssh',description:'A port on an SSH server'},
 {id:'remote_http',description:'Any HTTP or HTTPS URL'},
 {id:'ports_over_relay',description:'A service shared by another Zelport',destination:true}
];
export function defaultRouteStatus(route){
 const summary=route.summary||{};
 if(summary.state==='unavailable')return {tone:'warning',label:'Unavailable',detail:summary.detail||''};
 if(summary.state==='ready')return {tone:'ok',label:'Available',detail:summary.detail||''};
 if(summary.detail)return {tone:'warning',label:'Warning',detail:summary.detail};
 return {tone:'neutral',label:'Configured'};
}
export function createDashboard({root,modules,client,navigate,ui}){
 const rows=new Map();let table,body,empty;
 const enabled=module=>module?.descriptor.ui_placement==='settings'||client.state()?.module_availability?.[module?.descriptor.id]?.enabled!==false;
 function openAddService(trigger,{sourceId,domain}={}){
  const sources=sourceChoices.map(choice=>{const module=modules.get(choice.id);return {...choice,label:module?.descriptor.name||choice.id,available:!!module?.instance&&enabled(module)};}).filter(choice=>choice.available);
  const selected=sources.find(choice=>choice.id===sourceId&&!choice.destination)||sources.find(choice=>!choice.destination);
  if(!selected)return null;
  const module=modules.get(selected.id);
  return module.instance.edit(null,null,{sources,sourceId:selected.id,domain,trigger,
   onSource(id,draft){
    const choice=sources.find(item=>item.id===id),target=modules.get(id);if(!choice||!target)return;
    if(choice.destination){navigate(target.descriptor.tab_id);return;}
    openAddService(trigger,{sourceId:id,domain:draft.domain});
   },
   onSaved(){navigate('dashboard');}});
 }
 function addressText(state,route,address){if(state.remote_admin&&address){try{return new URL(address).host;}catch{}}return state.port===80?route.domain:`${route.domain}:${state.port}`;}
 function render(state=client.state()){
  const routes=Object.values(state.states).flatMap(module=>module.routes||[]);
  if(!table){
   const wrap=ui.el('div','dashboard-table-wrap');table=ui.el('table','dashboard-table data-table');
   const head=ui.el('tr'),thead=ui.el('thead');body=ui.el('tbody');
   for(const name of ['Address','Source','Target','Status','Actions']){const cell=ui.el('th',null,name);cell.scope='col';if(name==='Actions')cell.className='visually-hidden-label';head.append(cell);}
   thead.append(head);table.append(thead,body);wrap.append(table);empty=ui.el('div','empty dashboard-empty');root.append(wrap,empty);
  }
  table.parentElement.hidden=!routes.length;empty.hidden=!!routes.length;
  if(!routes.length)ui.renderChanged(empty,{modules:!!state.modules.length,port:state.port},()=>{
   empty.replaceChildren(ui.el('h2',null,state.modules.length?'No services yet':'Core only'),ui.el('p',null,state.modules.length?`Give a web service a stable name, for example my-app.localhost:${state.port}.`:'This build contains only the core. Add feature build tags to configure services.'));
  });
  const present=new Set(routes.map(route=>route.id));
  for(const [id,item] of rows)if(!present.has(id)){item.menu.dispose();item.row.remove();rows.delete(id);}
  for(const route of routes){
   let item=rows.get(route.id);
   if(!item){
    const row=ui.el('tr'),domain=ui.el('td','service-address'),source=ui.el('td','service-source'),target=ui.el('td','service-target'),status=ui.el('td','service-status-cell'),action=ui.el('td','service-actions-cell');
    for(const [cell,label]of [[domain,'Address'],[source,'Source'],[target,'Target'],[status,'Status'],[action,'Actions']])cell.dataset.label=label;
    const indicator=ui.serviceStatus(),actionRoot=ui.el('div','resource-actions dashboard-actions'),trigger=ui.el('button','resource-menu-trigger','⋮'),menu=ui.el('div','resource-menu');
    trigger.setAttribute('aria-haspopup','menu');menu.setAttribute('role','menu');actionRoot.append(trigger,menu);action.append(actionRoot);status.append(indicator.root);
    item={row,domain,source,target,indicator,trigger,route,menu:ui.disclosureMenu(actionRoot,trigger,menu)};rows.set(route.id,item);
    const open=ui.el('a','resource-menu-item','Open');open.setAttribute('role','menuitem');open.target='_blank';open.rel='noopener';open.addEventListener('click',()=>item.menu.close(true));
    const edit=ui.button('Edit',()=>{item.menu.close(true);modules.get(item.route.module)?.instance?.edit(item.route);},'resource-menu-item');
    const remove=ui.button('Remove',()=>{item.menu.close(true);modules.get(item.route.module)?.instance?.remove?.(item.route);},'resource-menu-item service-menu-remove');
    for(const control of [edit,remove])control.setAttribute('role','menuitem');
    menu.append(open,edit,remove);item.open=open;item.remove=remove;
    row.append(domain,source,target,status,action);body.append(row);
   }
   item.route=route;item.trigger.setAttribute('aria-label',`Actions for ${route.domain}`);
   const address=ui.serviceURL(state,route),linked=!!address,instance=modules.get(route.module)?.instance;
   if(!item.link||item.linked!==linked){item.link=ui.el(linked?'a':'span');item.linked=linked;item.domain.replaceChildren(item.link);if(linked){item.link.target='_blank';item.link.rel='noopener';}}
   ui.setText(item.link,addressText(state,route,address));if(linked&&item.address!==address){item.link.href=address;item.open.href=address;}item.address=address;item.open.hidden=!linked;
   item.remove.hidden=!instance?.remove;
   ui.setText(item.source,modules.get(route.module)?.descriptor.name||route.module);
   ui.setText(item.target,instance?.routeTarget?.(route)||route.summary.target||'');
   const status=instance?.routeStatus?.(route)||defaultRouteStatus(route);
   if(!linked&&status.tone==='neutral')Object.assign(status,{label:'Not connected',detail:'Not connected on this device.'});
   item.indicator.update(status);
  }
 }
 return {render,openAddService,dispose(){for(const item of rows.values())item.menu.dispose();rows.clear();}};
}
