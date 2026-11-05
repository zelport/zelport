import {setText,renderChanged,statusTone,showDialog,moduleViews,formFeedback} from '../internal/web/assets/ui.js';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import assert from 'node:assert/strict';
const source=new URL('../internal/modules/built_in_relay/assets/module.js',import.meta.url);
const {mount}=await import('data:text/javascript;base64,'+Buffer.from(readFileSync(source)).toString('base64'));
class El {
 constructor(tag,text){this.tag=tag;this.textContent=text;this.children=[];this.dataset={};this.events={};this.value='';}
 append(...children){for(const child of children){if(child.parentElement)child.parentElement.children=child.parentElement.children.filter(n=>n!==child);child.parentElement=this;this.children.push(child);}}
 addEventListener(type,cb){const previous=this.events[type];this.events[type]=previous?event=>{previous(event);return cb(event);}:cb;}
 setAttribute(name,value){this[name]=value;}
 close(){this.open=false;this.events.close?.();}
 remove(){this.removed=true;}
 showModal(){this.open=true;}
 getAttribute(name){return this[name];}
 removeAttribute(name){delete this[name];}
 focus(){document.activeElement=this;}
}
function harness(initialData={},renderInitially=true){
 const all=[],fields={};
 let latest={revision:1,data:{enabled:true,port:8090,bind_address:'',has_token:true,status:{state:'Listening',clients:0,listeners:[]},...initialData}};
 const ui={setTooltip(){},withInfo:control=>control,confirmation(){const root=new El('span');root.hidden=true;return {root,attach(){return this;},show(text){root.textContent=text;root.hidden=false;},clear(){root.textContent='';root.hidden=true;}};},busyButton(){},icon:()=>new El('svg'),setText,renderChanged,statusTone,showDialog,moduleViews,formFeedback,dialogHeader:(title)=>new El('h2',title),
  el:(tag,cls,text)=>{const e=new El(tag,text);all.push(e);return e;},
  field:(label,value,type)=>{const wrap=new El('label'),input=new El('input');input.value=value;input.checked=value;wrap.append(input);return fields[label]={wrap,input};},
  button:(text,cb)=>{const e=new El('button',text);e.click=cb;all.push(e);return e;},
  state:()=>({revision:latest.revision})
 };
 ui.picker=(label,options,value)=>{const control={wrap:new El('div'),value,dispose(){}};fields[label]=control;return control;};
 ui.adornedField=(label,value,type,options={})=>{const f=ui.field(label,value,type);f.wrap.append(...(options.actions||[]));return f;};
 ui.adornmentButton=(label,icon,fn)=>ui.button(label,fn);
 const ctx={ui,panel:new El('section'),moduleId:'relay',signal:new AbortController().signal};
 globalThis.document={body:new El('body'),createElement:tag=>new El(tag)};
 const m=mount(ctx);if(renderInitially)m.render(latest);m.activate();
 return {ctx,m,fields,all,form:all.find(e=>e.tag==='form'),render:(revision,data={})=>{latest={...latest,revision,data:{...latest.data,...data}};m.render(latest)}};
}

test('new relay settings render access-token protection off and save it explicitly',async()=>{
 const unrendered=harness({},false);
 assert.equal(unrendered.fields['Require access token'].input.checked,false);
 unrendered.m.dispose();
 const h=harness({enabled:false,has_token:false,require_access_token:false});
 h.fields.Port.input.value=8091;h.form.events.input({});
 assert.equal(h.fields['Require access token'].input.checked,false);
 let request;h.ctx.api=async(path,options)=>{request=JSON.parse(options.body);return {ok:true}};h.ctx.mutate=async work=>work();
 await h.form.events.submit({preventDefault(){}});
 assert.equal(request.require_access_token,false);
});

test('discard restores saved fields and revision after a conflicted draft',async()=>{
 const h=harness(),revisions=[];
 h.ctx.api=async(path,options)=>{revisions.push(JSON.parse(options.body).revision);throw Error('conflict')};
 h.ctx.mutate=async(work)=>{try{await work();}finally{h.render(2)}};
 h.ctx.refresh=async()=>h.render(2);
 h.fields.Port.input.value=8091;h.form.events.input({});await h.form.events.submit({preventDefault(){}});
 const reload=h.all.find(e=>e.textContent==='Discard');
 assert.ok(reload,'a conflicted draft needs a recovery action');
 await reload.click();assert.equal(h.fields.Port.input.value,'8090');h.fields.Port.input.value=8092;h.form.events.input({});await h.form.events.submit({preventDefault(){}});
 assert.deepEqual(revisions,[1,2]);
});

test('built-in relay keeps ordinary edits free of an unsaved-changes status',()=>{
 const h=harness();h.form.events.input({});
 assert.equal(h.all.some(e=>e.textContent==='Unsaved changes'),false);
});

for(const lifecycle of ['deactivate','dispose'])test(`late token response does not expose a secret after ${lifecycle}`,async()=>{
 const h=harness();let finish;
 h.ctx.api=()=>new Promise(resolve=>finish=resolve);
 const pending=h.all.find(e=>e.textContent==='Reveal access token').click();h.m[lifecycle]();finish({access_token:'test-only'});await pending;
 assert.equal(h.fields['Relay access token'].input.value,'');
});

test('edits made during save survive completion and polling',async()=>{
 const h=harness();let finish;
 h.ctx.api=()=>Promise.resolve({});
 h.ctx.mutate=async work=>{await work();await new Promise(resolve=>finish=resolve);h.render(2)};
 h.fields.Port.input.value=8091;h.form.events.input({});const pending=h.form.events.submit({preventDefault(){}});await Promise.resolve();
 h.fields.Port.input.value=9000;h.form.events.input({});finish();await pending;h.render(2);
 assert.equal(h.fields.Port.input.value,9000);
});

test('revealed token is erased on navigation and revision changes',async()=>{
 const h=harness();h.ctx.api=async()=>({access_token:'test-only'});
 await h.all.find(e=>e.textContent==='Reveal access token').click();
 assert.equal(h.fields['Relay access token'].input.value,'test-only');
 h.render(2);assert.equal(h.fields['Relay access token'].input.value,'');
 await h.all.find(e=>e.textContent==='Reveal access token').click();h.m.deactivate();
 assert.equal(h.fields['Relay access token'].input.value,'');
});

test('rotation after metadata polling preserves the draft revision for subsequent save',async()=>{
 const h=harness(),requests=[];
 h.fields.Port.input.value=9000;h.form.events.input({});h.render(2);
 h.ctx.api=async(path,options)=>{requests.push({path,...JSON.parse(options.body)});return {ok:true};};
 h.ctx.mutate=async(work)=>{await work();h.render(3);};
 await h.all.find(e=>e.textContent==='Change access token').click();
 h.fields['New token'].value='generate';h.fields['New token'].onchange();
 await h.all.filter(e=>e.tag==='form').at(-1).events.submit({preventDefault(){}});
 await h.form.events.submit({preventDefault(){}});
 assert.equal(requests[0].revision,1,'rotate using the draft revision, which accepts metadata-only changes');
 assert.equal(requests[1].revision,3,'successful rotation advances the surviving draft');
 assert.equal(requests[1].port,9000);
});

test('access protection and network settings are visible without an Advanced disclosure',async()=>{
 const h=harness({require_access_token:true});let request;
 assert.equal(h.all.some(e=>e.tag==='details'),false);
 assert.ok(h.form.children.some(e=>e.children?.includes(h.fields['Require access token'].wrap)));
 assert.ok(h.form.children.some(e=>e.children?.includes(h.fields['Bind address'].wrap)&&e.children?.includes(h.fields.Port.wrap)));
 h.fields['Require access token'].input.checked=false;h.form.events.input({});
 h.ctx.api=async(path,options)=>{request=JSON.parse(options.body);return {ok:true}};h.ctx.mutate=async(work)=>work();
 await h.form.events.submit({preventDefault(){}});assert.equal(request.require_access_token,false);
});

test('configuration keeps live relay status in Overview and aligns form actions with other settings',()=>{
 const h=harness(),connection=h.form.children.find(node=>node.children?.includes(h.fields['Enable relay'].wrap));
 assert.deepEqual(connection.children,[h.fields['Enable relay'].wrap]);
 const css=readFileSync(new URL('../internal/modules/built_in_relay/assets/style.css',import.meta.url),'utf8').replace(/\s+/g,'');
 assert.match(css,/\.relay-actions\{[^}]*justify-content:flex-end/);
});

test('clean Settings inputs and status text are not rewritten by unchanged polls',()=>{
 const h=harness();let writes=0;
 for(const [node,property] of [[h.fields.Port.input,'value'],...h.all.filter(e=>e.textContent==='Listening'||e.textContent==='0 connected clients').map(e=>[e,'textContent'])]){
  let value=String(node[property]);Object.defineProperty(node,property,{get(){return value},set(next){writes++;value=next}});
 }
 h.render(2);assert.equal(writes,0);
});

test('token rotation requires an explicit action before submitting',async()=>{
 const h=harness();let requests=0;
 h.ctx.api=async()=>{requests++;return {};};h.ctx.mutate=async work=>work();
 await h.all.find(e=>e.textContent==='Change access token').click();
 const form=h.all.filter(e=>e.tag==='form').at(-1),confirm=h.all.find(e=>e.textContent==='Change token');
 assert.equal(h.fields['New token'].value,'');assert.equal(confirm.disabled,true);
 await form.events.submit({preventDefault(){}});assert.equal(requests,0);
 h.fields['New token'].value='replace';h.fields['New token'].onchange();
 assert.equal(confirm.disabled,false);assert.equal(h.fields['New access token'].wrap.hidden,false);
 assert.equal(h.fields['New access token'].input.required,true);
});

test('leaving Built-in Relay clears revealed and pending tokens',async()=>{
 const h=harness();h.m.openSettings();h.ctx.api=async()=>({access_token:'test-only'});
 await h.all.find(e=>e.textContent==='Reveal access token').click();assert.equal(h.fields['Relay access token'].input.value,'test-only');
 h.m.deactivate();assert.equal(h.fields['Relay access token'].input.value,'');
 h.m.openSettings();let finish;h.ctx.api=()=>new Promise(resolve=>finish=resolve);const pending=h.all.find(e=>e.textContent==='Reveal access token').click();
 h.m.deactivate();finish({access_token:'late-test-only'});await pending;assert.equal(h.fields['Relay access token'].input.value,'');h.m.dispose();
});

test('relay Save and Discard follow actual changes, including reverting a field',()=>{
 const h=harness(),save=h.all.find(n=>n.textContent==='Save'),discard=h.all.find(n=>n.textContent==='Discard');
 assert.equal(save.disabled,true);assert.equal(discard.disabled,true);
 h.fields.Port.input.value='8091';h.form.events.input({});assert.equal(save.disabled,false);assert.equal(discard.disabled,false);
 h.fields.Port.input.value='8090';h.form.events.input({});assert.equal(save.disabled,true);assert.equal(discard.disabled,true);
});

test('reverting to submitted values while save completes leaves a clean form',async()=>{
 const h=harness();let finish;
 h.ctx.api=()=>Promise.resolve({});
 h.ctx.mutate=async work=>{await work();await new Promise(resolve=>finish=resolve);h.render(2,{port:8091})};
 h.fields.Port.input.value='8091';h.form.events.input({});
 const pending=h.form.events.submit({preventDefault(){}});await Promise.resolve();
 h.fields.Port.input.value='8092';h.form.events.input({});
 h.fields.Port.input.value='8091';h.form.events.input({});
 finish();await pending;h.render(2);
 assert.equal(h.fields.Port.input.value,'8091');
 assert.equal(h.all.find(n=>n.textContent==='Save').disabled,true,'no remaining change to save');
});
