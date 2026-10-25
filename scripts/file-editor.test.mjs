import test from 'node:test';
import assert from 'node:assert/strict';
import {fileEditor} from '../internal/web/assets/ui.js';

class Node{
 constructor(tag='div'){this.tag=tag;this.tagName=tag.toUpperCase();this.children=[];this.events=new Map();this.attrs={};this.value='';this.textContent='';this.hidden=false;this.disabled=false;this.style={};this.dataset={};if(tag==='textarea'){let text='';Object.defineProperty(this,'value',{get(){return text;},set(v){text=String(v).replace(/\r\n?/g,'\n');}});}}
 append(...nodes){for(const n of nodes){n.remove();n.parentElement=this;this.children.push(n);}}
 setAttribute(k,v){this.attrs[k]=String(v);} getAttribute(k){return this.attrs[k];}
 addEventListener(k,f){this.events.set(k,[...(this.events.get(k)||[]),f]);} removeEventListener(k,f){this.events.set(k,(this.events.get(k)||[]).filter(n=>n!==f));}
 dispatchEvent(e){for(const f of this.events.get(e.type)||[])f(e);return true;}
 remove(){if(this.parentElement)this.parentElement.children=this.parentElement.children.filter(n=>n!==this);this.parentElement=null;}
 focus(){document.activeElement=this;} showModal(){this.open=true;} close(){this.open=false;this.dispatchEvent({type:'close'});}
}
const tree=n=>[n,...n.children.flatMap(tree)],tick=()=>new Promise(r=>setImmediate(r));
function fire(n,type,extra={}){assert.ok(n);n.dispatchEvent({type,preventDefault(){this.defaultPrevented=true;},...extra});}
function setup(content='Host original\n',loadError=null){
 const body=new Node('body'),listeners=new Map();globalThis.document={body,createElement:t=>new Node(t),activeElement:null};globalThis.window={addEventListener(k,f){listeners.set(k,f);},removeEventListener(k){listeners.delete(k);}};
 const requests=[],controller=new AbortController();let saved=0;
 const ctx={signal:controller.signal,ui:{state:()=>({revision:9,remote_admin:{hostname:'remote-test',device_id:'peer'}})},api:async(path,options={})=>{requests.push({path,...options,body:options.body&&JSON.parse(options.body)});if(loadError)throw loadError;return {path:'/test/config',content,version:'v1',revision:7};}};
 const editor=fileEditor(ctx,{endpoint:'/api/modules/test/config-file',title:'Edit SSH configuration',onSaved:async()=>{saved++;}});
 const all=()=>tree(body),find=text=>all().find(n=>n.textContent===text),textarea=()=>all().find(n=>n.tag==='textarea');
 return {ctx,editor,controller,requests,listeners,all,find,textarea,saved:()=>saved};
}
test('file editor loads only on opening and saves loaded version and revision, not poll revision',async()=>{
 const h=setup();await tick();const area=h.textarea();assert.equal(area.value,'Host original\n');assert.ok(h.find('remote-test'));area.value='Host edited\n';fire(area,'input');fire(h.find('Save'),'click');await tick();
 assert.deepEqual(h.requests[1],{path:'/api/modules/test/config-file',method:'PUT',body:{revision:7,version:'v1',content:'Host edited\n'}});assert.equal(h.saved(),1);assert.equal(h.textarea(),undefined);assert.equal(h.listeners.size,0);
});
test('errors and conflicts keep drafts and prevent blind overwrite retries',async()=>{
 const h=setup();await tick();const area=h.textarea();area.value='draft';area.selectionStart=2;fire(area,'input');h.ctx.api=async()=>{throw Object.assign(Error('file changed'),{status:409});};fire(h.find('Save'),'click');await tick();assert.equal(h.textarea(),area);assert.equal(area.value,'draft');assert.equal(area.selectionStart,2);assert.equal(h.find('Save').disabled,true);assert.ok(h.find('Reload file'));assert.equal(document.activeElement.getAttribute('role'),'alert');fire(h.find('Cancel'),'click');assert.ok(h.find('Discard changes'));fire(h.find('Keep editing'),'click');assert.equal(area.value,'draft');h.editor.dispose();
});
test('dirty cancel and Escape require explicit discard; clean cancel closes',async()=>{
 const h=setup();await tick();h.textarea().value='draft';fire(h.textarea(),'input');fire(h.all().find(n=>n.tag==='dialog'),'cancel');assert.ok(h.textarea());fire(h.find('Discard changes'),'click');assert.equal(h.textarea(),undefined);
 const clean=setup();await tick();fire(clean.find('Cancel'),'click');assert.equal(clean.textarea(),undefined);
});
test('keyboard save serializes requests and closing is blocked while writing',async()=>{
 const h=setup();await tick();const area=h.textarea();area.value='saved';fire(area,'input');let finish,calls=0;h.ctx.api=()=>{calls++;return new Promise(r=>finish=r);};fire(area,'keydown',{key:'s',ctrlKey:true});fire(area,'keydown',{key:'s',ctrlKey:true});fire(h.find('Cancel'),'click');assert.equal(calls,1);assert.equal(area.disabled,true);assert.ok(h.textarea());finish({});await tick();assert.equal(h.textarea(),undefined);
});
test('invalid files remain editable and reload failure preserves draft',async()=>{
 const h=setup();await tick();const area=h.textarea();area.value='invalid PEM';fire(area,'input');h.ctx.api=async()=>{throw Error('Use certificates only');};fire(h.find('Save'),'click');await tick();assert.equal(area.value,'invalid PEM');assert.equal(h.find('Save').disabled,false);assert.equal(document.activeElement,h.find('Use certificates only'));h.editor.dispose();
});


test('CRLF files start clean and retain their newline convention after editing',async()=>{
 const h=setup('Host original\r\n');await tick();assert.equal(h.textarea().value,'Host original\n');assert.equal(h.find('Save').disabled,true);
 h.textarea().value='Host changed\n';fire(h.textarea(),'input');fire(h.find('Save'),'click');await tick();assert.equal(h.requests[1].body.content,'Host changed\r\n');
});

test('failed explicit reload preserves the old draft after a conflict',async()=>{
 const h=setup();await tick();h.textarea().value='copyable draft';fire(h.textarea(),'input');h.ctx.api=async()=>{throw Object.assign(Error('Conflict'),{status:409});};fire(h.find('Save'),'click');await tick();fire(h.find('Reload file'),'click');fire(h.find('Discard changes'),'click');await tick();assert.equal(h.textarea().value,'copyable draft');assert.equal(h.find('Save').disabled,true);h.editor.dispose();
});


test('an initial read failure cannot accept a draft that cannot be saved',async()=>{
 const h=setup('',Error('File unavailable'));await tick();assert.equal(h.textarea().disabled,true);assert.equal(h.find('Save').disabled,true);assert.ok(h.find('Reload file'));assert.equal(document.activeElement.getAttribute('role'),'alert');fire(h.find('Cancel'),'click');assert.equal(h.textarea(),undefined);
});
