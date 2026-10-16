import test from 'node:test';
import assert from 'node:assert/strict';
import {mount} from '../internal/modules/remote_http/assets/module.js';

test('Remote HTTP keeps the trust notice and updates warnings without losing form state',()=>{
 let definition;const fields=new Map();
 const el=(tag,cls,text='')=>({tag,cls,textContent:text,children:[],append(...nodes){this.children.push(...nodes);},setAttribute(){}});
 const field=(label,value,type)=>{const input={value,checked:type==='checkbox'?value:false,events:{},addEventListener(k,fn){this.events[k]=fn;}};const result={input,wrap:el('div')};fields.set(label,result);return result;};
 mount({ui:{el,field,serviceModule:(ctx,d)=>{definition=d;}}});
 const form=definition.fields({target_url:'https://example.test/service',rewrite_origin:true});
 const [notice,,,warning]=form.element.children;
 assert.equal(notice.textContent,'Only proxy services you trust. A localhost secure context does not guarantee a secure upstream connection. Avoid banking and other sensitive websites.');
 assert.equal(warning.hidden,true);
 const target=fields.get('Target URL').input,verify=fields.get('Verify TLS certificate');
 verify.input.checked=false;verify.input.events.change();
 assert.match(warning.textContent,/identity is not verified/);assert.equal(warning.hidden,false);assert.equal(form.read().skip_tls_verify,true);
 target.value='HTTP://example.test/service';target.events.input();assert.match(warning.textContent,/Unencrypted/);assert.equal(verify.wrap.hidden,true);assert.equal(form.read().skip_tls_verify,false);
 target.value='https://example.test/other';target.events.input();assert.equal(verify.input.checked,false);assert.equal(form.read().rewrite_origin,true);
 verify.input.checked=true;verify.input.events.change();assert.equal(warning.hidden,true);
 target.value='';target.events.input();assert.equal(warning.hidden,true);assert.equal(form.element.children[0],notice);
});
