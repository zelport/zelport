import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';
import {mount} from '../internal/modules/remote_http/assets/module.js';

class Node{
 constructor(tag){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.dataset={};this.style={};this.attrs={};this.events={};this.className='';this.textContent='';this.hidden=false;this.value='';}
 append(...nodes){for(const node of nodes){node.parentElement=this;this.children.push(node);}}
 prepend(...nodes){this.children.unshift(...nodes);}
 replaceChildren(...nodes){this.children=[];this.append(...nodes);}
 setAttribute(name,value){this.attrs[name]=String(value);}getAttribute(name){return this.attrs[name];}removeAttribute(name){delete this.attrs[name];}
 addEventListener(name,fn){this.events[name]=fn;}removeEventListener(){}
}
const tree=node=>[node,...node.children.flatMap(tree)];

test('Remote HTTP shows one trust hint, swaps it for a warning when needed and keeps form state',()=>{
 globalThis.document={createElement:tag=>new Node(tag),createElementNS:(_,tag)=>new Node(tag)};
 let definition;
 mount({ui:{...ui,moduleViews:()=>({overview:new Node('section'),configuration:new Node('section'),actions:new Node('div'),show(){},dispose(){}}),serviceModule:(ctx,d)=>{definition=d;return {render(){},dispose(){}};}},panel:new Node('section'),api:async()=>({}),mutate:work=>work(),signal:new AbortController().signal});
 const form=definition.fields({target_url:'https://example.test/service',rewrite_origin:true}),nodes=tree(form.element);
 const hint=nodes.find(n=>n.className.includes('remote-trust-notice')),warning=nodes.find(n=>n.className.includes('field-warning'));
 const inputs=nodes.filter(n=>n.tag==='input'),target=inputs.find(n=>n.type==='url'),[verify,origin]=inputs.filter(n=>n.type==='checkbox');
 const verifyRow=nodes.filter(n=>n.className==='option-row')[0];
 assert.equal(hint.textContent,'Only for services you trust.');assert.equal(hint.hidden,false);assert.equal(warning.hidden,true);
 assert.ok(nodes.some(n=>n.className==='service-options'),'switches sit in the shared options group');
 verify.checked=false;verify.events.change();
 assert.match(warning.textContent,/certificate is not verified/);assert.equal(warning.hidden,false);assert.equal(hint.hidden,true,'the warning replaces the hint');assert.equal(form.read().skip_tls_verify,true);
 target.value='HTTP://example.test/service';target.events.input();assert.match(warning.textContent,/unencrypted/);assert.equal(verifyRow.hidden,true);assert.equal(form.read().skip_tls_verify,false);
 target.value='https://example.test/other';target.events.input();assert.equal(verify.checked,false);assert.equal(origin.checked,true);assert.equal(form.read().rewrite_origin,true);
 verify.checked=true;verify.events.change();assert.equal(warning.hidden,true);assert.equal(hint.hidden,false);
});
