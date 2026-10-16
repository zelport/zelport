const trustNotice='Only proxy services you trust. A localhost secure context does not guarantee a secure upstream connection. Avoid banking and other sensitive websites.';
export function mount(ctx){
 const {el,field,serviceModule}=ctx.ui;
 return serviceModule(ctx,{
  empty:'No remote HTTP services yet. Add a domain and a target URL.',
  fields(s){
   const element=el('div'),target=field('Target URL',s.target_url||'','url'),verify=field('Verify TLS certificate',!s.skip_tls_verify,'checkbox'),origin=field('Rewrite Origin',s.rewrite_origin,'checkbox');
   target.input.required=true;target.input.placeholder='https://example.com/service01';
   const notice=el('p','small remote-trust-notice',trustNotice),warning=el('p','small route-error'),hint=el('p','small muted','Use the target origin for HTTP and WebSocket requests from this domain.');
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
}
