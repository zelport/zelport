// Sources lists each service source once: its key facts and a fixed Configure action.
export function createSources({root,modules,client,navigate,ui}){
 const cards=new Map();
 const enabled=module=>client.state()?.module_availability?.[module.descriptor.id]?.enabled!==false;
 function render(state=client.state()){
  if(!state)return;
  const list=[...modules.values()].filter(m=>m.descriptor.ui_placement!=='settings'&&enabled(m)).sort((a,b)=>a.descriptor.order-b.descriptor.order);
  const ids=new Set(list.map(m=>m.descriptor.id));
  for(const [id,card]of cards)if(!ids.has(id)){card.root.remove();cards.delete(id);}
  if(!root.classList?.contains('source-grid'))root.className='source-grid';
  for(const m of list){
   const {id,name,maturity,tab_id}=m.descriptor;let card=cards.get(id);
   if(!card){
    const box=ui.el('section','source-card'),head=ui.el('div','source-card-heading'),title=ui.el('h2',null,name),facts=ui.el('dl','source-facts');
    title.id=`source-${id}`;box.setAttribute('aria-labelledby',title.id);head.append(title);if(maturity==='experimental')head.append(ui.experimentalBadge());
    const configure=ui.button('Configure',()=>navigate(tab_id),'secondary source-configure');configure.setAttribute('aria-label',`Configure ${name}`);
    box.append(head,facts,configure);root.append(box);card={root:box,facts};cards.set(id,card);
   }
   const moduleState=state.states?.[id]||{},count=(moduleState.routes||[]).length;
   const rows=[{label:'Services',value:String(count)},...(m.sourceFacts?.(moduleState)||[])];
   ui.renderChanged(card.facts,rows,()=>{card.facts.replaceChildren(...rows.map(({label,value,tone})=>{const row=ui.el('div'),dd=ui.el('dd',null,value);if(tone)dd.dataset.tone=tone;row.append(ui.el('dt',null,label),dd);return row;}));});
   root.append(card.root);
  }
 }
 return {render};
}
