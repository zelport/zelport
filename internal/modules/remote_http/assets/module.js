import {mountConfiguration} from './configuration.js';

const trustNotice='Only for services you trust.';
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
   const {el,field,fieldHint,optionSwitch,serviceOptions}=ctx.ui;
   const element=el('div'),target=field('Target URL',s.target_url||'','url'),options=serviceOptions();
   const verify=optionSwitch('Verify TLS certificate',!s.skip_tls_verify),origin=optionSwitch('Use target origin',s.rewrite_origin,'For services that reject requests from localhost.');
   target.input.required=true;target.input.placeholder='https://example.com';target.input.autocomplete='off';target.input.spellcheck=false;
   const hint=fieldHint(trustNotice,'remote-trust-notice'),warning=fieldHint('','field-warning');warning.setAttribute('role','status');target.wrap.append(hint,warning);
   const update=()=>{
    let protocol='';try{protocol=new URL(target.input.value.trim()).protocol;}catch{}
    verify.row.hidden=protocol!=='https:';
    warning.textContent=protocol==='http:'?'Traffic to this target is unencrypted.':protocol==='https:'&&!verify.input.checked?"The server's certificate is not verified.":'';
    warning.hidden=!warning.textContent;hint.hidden=!warning.hidden;
   };
   target.input.addEventListener('input',update);verify.input.addEventListener('change',update);
   options.append(verify.row,origin.row);element.append(target.wrap,options);update();
   return {element,read:()=>({target_url:target.input.value.trim(),skip_tls_verify:!verify.row.hidden&&!verify.input.checked,rewrite_origin:origin.input.checked})};
  }
 });
 return {...routes,routeStatus(){return {tone:'neutral',label:'Not checked',detail:'Zelport does not check remote URLs.'};},routeWarning(route){return route.summary.detail||'';},render(state){routes.render(state);settings.render(state);settings.open(false);},openSettings(){views.show('configuration');settings.open();},dispose(){settings.dispose();routes.dispose();views.dispose();}};
}
