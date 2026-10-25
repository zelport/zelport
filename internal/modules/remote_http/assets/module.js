import {mountConfiguration} from './configuration.js';

const trustNotice='Only proxy services you trust. A localhost secure context does not guarantee a secure upstream connection. Avoid banking and other sensitive websites.';
export function mount(ctx){
 const {el,field,serviceModule,warningIndicator}=ctx.ui;
 const views=ctx.ui.moduleViews({...ctx,moduleName:ctx.moduleName||'Remote HTTP'},{onChange(view){if(view==='configuration')settings.open();}});
 const settings=mountConfiguration(ctx,views.configuration);
 const routes=serviceModule({...ctx,panel:views.overview,toolbar:views.actions},{
  routeMenu:true,
  columns:['Service','Target URL','Info','Actions'],
  tableLabel:'Remote HTTP services',
  serviceLabel:(route,state)=>`${route.domain}:${state.port}`,
  targetLabel:route=>route.editor?.target_url||route.summary.target,
  targetInfo:route=>`Target URL: ${route.editor?.target_url||route.summary.target}`,
  rowIndicators:route=>route.summary.detail?[warningIndicator(route.summary.detail).root]:[],
  emptyState:{kind:'not-configured',message:'No remote HTTP services yet.',actionLabel:'+ Add service'},
  fields(s){
   const element=el('div'),target=field('Target URL',s.target_url||'','url'),verify=field('Verify TLS certificate',!s.skip_tls_verify,'checkbox'),origin=field('Use target origin',s.rewrite_origin,'checkbox');
   target.input.required=true;target.input.placeholder='https://example.com/service01';
   const notice=el('p','small remote-trust-notice',trustNotice),warning=el('p','small route-error'),hint=el('p','small muted','Send the target URL’s Origin header with HTTP and WebSocket requests. Enable this if the service rejects requests from its localhost address.');
   warning.setAttribute('role','status');
   const update=()=>{
    let protocol='';try{protocol=new URL(target.input.value.trim()).protocol;}catch{}
    verify.wrap.hidden=protocol!=='https:';
    warning.textContent=protocol==='http:'?'Unencrypted upstream connection. Traffic can be read or modified in transit.':protocol==='https:'&&!verify.input.checked?"TLS certificate verification is disabled. The remote server's identity is not verified.":'';
    warning.hidden=!warning.textContent;
   };
   target.input.addEventListener('input',update);verify.input.addEventListener('change',update);
   element.append(notice,target.wrap,verify.wrap,warning,origin.wrap,hint);update();
   return {element,read:()=>({target_url:target.input.value.trim(),skip_tls_verify:!verify.wrap.hidden&&!verify.input.checked,rewrite_origin:origin.input.checked})};
  }
 });
 return {...routes,render(state){routes.render(state);settings.render(state);},openOverview(){views.show('overview');},openSettings(){views.show('configuration');settings.open();},dispose(){settings.dispose();routes.dispose();views.dispose();}};
}
