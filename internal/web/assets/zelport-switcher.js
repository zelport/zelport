// Zelport switcher: the chip at the right of the header names where you are.
// It appears only when relay links this Zelport to others (or while administering one).
// Linked Zelports open their administration in a new tab; offline ones are listed but inactive.
const external='M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5';
function svg(path,className){
 const node=document.createElementNS('http://www.w3.org/2000/svg','svg'),line=document.createElementNS('http://www.w3.org/2000/svg','path');
 node.setAttribute('viewBox','0 0 24 24');node.setAttribute('aria-hidden','true');node.setAttribute('focusable','false');node.setAttribute('class',className);line.setAttribute('d',path);node.append(line);return node;
}
const laptop='M4 6a1 1 0 0 1 1-1h14a1 1 0 0 1 1 1v9H4zM2 19h20';
const linked='M6 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM18 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM18 15a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM8.6 10.6l6.8-3.2M8.6 13.4l6.8 3.2';
// The local administration of this computer, seen from a remote administration tab.
export function localAdministrationURL(location){return `${location.protocol}//localhost${location.port?`:${location.port}`:''}/`;}
export function switcherModel(state){
 const relay=state.states?.ports_over_relay?.data||{};
 if(state.remote_admin)return {mode:'remote',hostname:state.remote_admin.hostname||state.remote_admin.device_id,peers:[]};
 const enabled=state.module_availability?.ports_over_relay?.enabled!==false&&relay.enabled===true;
 if(!enabled)return null;
 const peers=(relay.administration_peers||[]).map(peer=>({hostname:peer.hostname||peer.device_id,url:peer.hostname_url||peer.uuid_url,available:!!peer.available})).sort((a,b)=>a.hostname.localeCompare(b.hostname));
 return {mode:'local',hostname:state.runtime?.hostname||'',peers};
}
export function createSwitcher({host,ui,location=globalThis.location}){
 const root=ui.el('div','zelport-switcher'),trigger=ui.el('button','zelport-chip'),menu=ui.el('div','zelport-menu');
 menu.setAttribute('role','menu');trigger.setAttribute('aria-haspopup','menu');root.append(trigger,menu);root.hidden=true;host.append(root);
 const disclosure=ui.disclosureMenu(root,trigger,menu);let drawn='';
 function label(text){const node=ui.el('p','zelport-menu-label',text);node.disabled=true;node.setAttribute('role','presentation');return node;}
 function dot(tone){const slot=ui.el('span','zelport-menu-dot');slot.dataset.tone=tone;return slot;}
 function current(text){const row=ui.el('div','zelport-menu-item');row.setAttribute('role','menuitem');row.setAttribute('aria-current','true');row.tabIndex=-1;row.append(dot('ok'),ui.el('strong',null,text),ui.el('span','zelport-menu-check','✓'));return row;}
 function link(text,href,name){const row=ui.el('a','zelport-menu-item');row.setAttribute('role','menuitem');row.href=href;row.target='_blank';row.rel='noopener';row.setAttribute('aria-label',`${name}, opens in a new tab`);row.title=name;row.append(dot('ok'),ui.el('span',null,text),svg(external,'zelport-menu-external'));row.addEventListener('click',()=>disclosure.close());return row;}
 function render(state){
  const model=switcherModel(state);root.hidden=!model;if(!model){disclosure.close();return;}
  const key=JSON.stringify(model);if(key===drawn)return;drawn=key;
  trigger.classList.toggle('remote',model.mode==='remote');
  if(model.mode==='remote'){
   trigger.replaceChildren(svg(linked,'zelport-chip-icon'),ui.el('span','zelport-chip-name',model.hostname),ui.el('span','zelport-chip-caret','▾'));
   trigger.setAttribute('aria-label',`Zelport: administering ${model.hostname}`);trigger.removeAttribute('title');
   // Leaving returns this tab to this computer's own administration. The remote page is only loaded through the relay.
   const exit=ui.el('button','zelport-menu-item zelport-menu-exit','Exit remote administration');exit.type='button';exit.setAttribute('role','menuitem');
   exit.addEventListener('click',()=>{disclosure.close();location.assign(localAdministrationURL(location));});
   menu.replaceChildren(label('Administering'),current(model.hostname),ui.el('div','zelport-menu-separator'),exit);
  }else{
   trigger.replaceChildren(svg(laptop,'zelport-chip-icon'),ui.el('span','zelport-chip-caret','▾'));
   trigger.setAttribute('aria-label',`Zelport: this Zelport, ${model.hostname}`);trigger.title=`This Zelport · ${model.hostname}`;
   const rows=[label('This Zelport'),current(model.hostname)];
   if(model.peers.length)rows.push(label('Linked Zelports'));
   for(const peer of model.peers){
    if(peer.available&&peer.url){rows.push(link(peer.hostname,peer.url,`Administer ${peer.hostname}`));continue;}
    const row=ui.el('div','zelport-menu-item');row.setAttribute('role','menuitem');row.setAttribute('aria-disabled','true');row.disabled=true;row.dataset.offline='true';
    row.append(dot('off'),ui.el('span',null,peer.hostname),ui.el('span','zelport-menu-note','Offline'));rows.push(row);
   }
   menu.replaceChildren(...rows);
  }
 }
 return {render,dispose(){disclosure.dispose();root.remove();}};
}
// Remote administration banner: where the actions run. Leaving is in the Zelport switcher.
export function renderRemoteBanner({banner,state,ui}){
 banner.hidden=!state.remote_admin;if(!state.remote_admin)return;
 const name=state.remote_admin.hostname||state.remote_admin.device_id;
 ui.renderChanged(banner,[name],()=>{
  const text=ui.el('span','remote-administration-text');text.append('Administering ',ui.el('strong',null,name));
  banner.replaceChildren(text);
 });
}
