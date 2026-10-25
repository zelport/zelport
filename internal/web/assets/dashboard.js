export function createDashboard({root,modules,client,navigate,ui}){
 const rows=new Map();let table,body,empty;
 const enabled=module=>module?.descriptor.ui_placement==='settings'||client.state()?.module_availability?.[module?.descriptor.id]?.enabled!==false;
 function openAddService(trigger){
  const drafts=new Map();
  const choose=()=>ui.serviceLocationDialog({trigger,options:[
   option('This computer','local_ports','Local Ports connects a service running on this computer.'),
   option('Development server','ports_over_ssh','Ports over SSH connects a service on a server using your existing SSH configuration.'),
   option('Remote HTTP','remote_http','Connect an existing HTTP or HTTPS service by URL.',true),
   option('Ports over Relay','ports_over_relay','Discover services shared by your other Zelport devices.',true,true)
  ]});
  function option(label,id,description,secondary=false,destination=false){
   const module=modules.get(id),available=!!module?.instance&&enabled(module),reason=!module?.instance?'Not included in this build.':'Turn on this service source in Settings.';
   return {label,description,secondary,available,reason,run(){
    if(!available)return;
    if(destination){drafts.clear();navigate(module.descriptor.tab_id);module.instance.openSettings?.();return;}
    module.instance.edit(null,null,{draft:drafts.get(id),onBack(draft){drafts.set(id,draft);choose();},onSaved(){drafts.clear();navigate(module.descriptor.tab_id);}});
   }};
  }
  return choose();
 }
 function render(state=client.state()){
  const routes=Object.values(state.states).flatMap(module=>module.routes||[]);
  if(!table){const wrap=ui.el('div','dashboard-table-wrap');table=ui.el('table','dashboard-table data-table');const head=ui.el('tr'),thead=ui.el('thead');body=ui.el('tbody');for(const name of ['Domain','Source type','Data flow','Actions']){const cell=ui.el('th',null,name);cell.scope='col';head.append(cell);}thead.append(head);table.append(thead,body);wrap.append(table);empty=ui.el('div','empty dashboard-empty');root.append(wrap,empty);}
  table.parentElement.hidden=!routes.length;empty.hidden=!!routes.length;if(!routes.length)ui.renderChanged(empty,!!state.modules.length,()=>{empty.replaceChildren(ui.el('h2',null,state.modules.length?'Give your service a localhost address':'Core only'),ui.el('p',null,state.modules.length?'Add a web service from this computer or a development server. Open it through a named localhost address, with support for WebSockets and secure-context browser features.':'This build contains only the core. Add feature build tags to configure services.'));if(state.modules.length)empty.append(ui.el('p','dashboard-example',`For example: port 3000 → http://my-app.localhost:${state.port}`));});
  const present=new Set(routes.map(route=>route.id));for(const [id,item] of rows)if(!present.has(id)){item.menu.dispose();item.row.remove();rows.delete(id);}
  for(const route of routes){let item=rows.get(route.id);if(!item){const row=ui.el('tr'),domain=ui.el('td'),source=ui.el('td'),target=ui.el('td'),action=ui.el('td');for(const [cell,label]of [[domain,'Domain'],[source,'Source type'],[target,'Data flow'],[action,'Actions']])cell.dataset.label=label;const targetText=ui.el('span'),notice=ui.warningIndicator('','Service warning'),actionRoot=ui.el('div','resource-actions dashboard-actions'),trigger=ui.el('button','resource-menu-trigger','⋮'),menu=ui.el('div','resource-menu');trigger.setAttribute('aria-label',`Actions for ${route.domain}`);trigger.setAttribute('aria-haspopup','menu');menu.setAttribute('role','menu');actionRoot.append(notice.root,trigger,menu);action.append(actionRoot);target.append(targetText);item={row,domain,source,target,targetText,notice,trigger,route,menu:ui.disclosureMenu(actionRoot,trigger,menu)};rows.set(route.id,item);const edit=ui.button('Edit',()=>{item.menu.close(true);modules.get(item.route.module)?.instance?.edit(item.route);},'resource-menu-item');edit.setAttribute('role','menuitem');menu.append(edit);row.append(domain,source,target,action);body.append(row);}item.route=route;item.trigger.setAttribute('aria-label',`Actions for ${route.domain}`);const address=ui.serviceURL(state,route),linked=!!address;if(!item.link||item.linked!==linked){item.link=ui.el(linked?'a':'span');item.linked=linked;item.domain.replaceChildren(item.link);if(linked){item.link.target='_blank';item.link.rel='noopener';}else item.domain.append(ui.el('span','small muted','Not connected on this device'));}ui.setText(item.link,route.domain);if(linked&&item.address!==address)item.link.href=address;item.address=address;ui.setText(item.source,modules.get(route.module)?.descriptor.name||route.module);ui.setText(item.targetText,route.summary.target||'');const notice=route.summary.detail||modules.get(route.module)?.instance?.routeNotice?.(route)||'';item.notice.update(notice);}
 }
 return {render,openAddService,dispose(){for(const item of rows.values())item.menu.dispose();rows.clear();}};
}
