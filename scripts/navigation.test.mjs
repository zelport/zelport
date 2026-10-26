import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import * as ui from '../internal/web/assets/ui.js';

class Node {
 constructor(){this.children=[];this.dataset={};this.attrs={};this.events={};this.hidden=false;this.width=0;this.style={};}
 append(...nodes){for(const n of nodes){if(n.parentElement)n.parentElement.children=n.parentElement.children.filter(x=>x!==n);this.children.push(n);n.parentElement=this;}}
 prepend(n){if(n.parentElement)n.parentElement.children=n.parentElement.children.filter(x=>x!==n);this.children.unshift(n);n.parentElement=this;}
 setAttribute(k,v){this.attrs[k]=v;}
 getAttribute(k){return this.attrs[k];}
 removeAttribute(k){delete this.attrs[k];}
 addEventListener(k,v){const previous=this.events[k];this.events[k]=previous?event=>{previous(event);v(event);}:v;}
 removeEventListener(){}
 getBoundingClientRect(){return {width:this.width};}
 querySelectorAll(){return this.children;}
 contains(n){return n===this||this.children.includes(n);}
 cloneNode(){const n=new Node();n.width=this.width;return n;}
 remove(){}
 focus(){document.activeElement=this;}
}
function setup(){
 globalThis.document={body:new Node(),activeElement:null,createElement:()=>new Node(),addEventListener(){},removeEventListener(){}};
 globalThis.MutationObserver=class{observe(){}disconnect(){}};
}
test('Services groups module buttons and preserves the Dashboard and Settings controls',()=>{
 setup();const tabs=new Node(),dashboard=new Node(),module=new Node(),settings=new Node();
 dashboard.id='tab-dashboard';settings.id='tab-settings';module.id='tab-ssh';module.textContent='Ports over SSH';module.setAttribute('aria-current','page');
 tabs.append(dashboard,module,settings);
 const nav=ui.serviceNavigation(tabs);
 assert.equal(tabs.children[0],dashboard);assert.equal(tabs.children.at(-1),settings);
 assert.equal(module.parentElement,nav.menu);assert.equal(nav.trigger.dataset.selected,'true');
 nav.trigger.events.click({detail:0,preventDefault(){}});assert.equal(nav.menu.hidden,false);assert.equal(document.activeElement,module);
 nav.menu.events.keydown({key:'Escape',preventDefault(){},stopPropagation(){}});assert.equal(nav.menu.hidden,true);assert.equal(document.activeElement,nav.trigger);
 module.hidden=true;nav.select();assert.equal(nav.trigger.disabled,true);
});

test('Services keyboard navigation skips hidden modules and preserves current selection',()=>{
 setup();const tabs=new Node(),dashboard=new Node(),a=new Node(),hidden=new Node(),b=new Node(),settings=new Node();
 dashboard.id='tab-dashboard';settings.id='tab-settings';hidden.hidden=true;a.setAttribute('aria-current','page');tabs.append(dashboard,a,hidden,b,settings);
 const nav=ui.serviceNavigation(tabs);nav.trigger.events.click({detail:0,preventDefault(){}});
 nav.menu.events.keydown({key:'ArrowDown',preventDefault(){}});assert.equal(document.activeElement,b);
 nav.menu.events.keydown({key:'Home',preventDefault(){}});assert.equal(document.activeElement,a);
 assert.equal(a.getAttribute('aria-current'),'page');nav.select();assert.equal(nav.menu.hidden,true);assert.equal(document.activeElement,nav.trigger);
});
test('core-only navigation keeps Services disabled',()=>{
 setup();const tabs=new Node(),dashboard=new Node(),settings=new Node();dashboard.id='tab-dashboard';settings.id='tab-settings';tabs.append(dashboard,settings);
 const nav=ui.serviceNavigation(tabs);assert.equal(nav.trigger.disabled,true);assert.equal(nav.menu.hidden,true);nav.dispose();
});

test('pointer opening does not move focus into the Services menu',()=>{
 setup();const tabs=new Node(),dashboard=new Node(),module=new Node(),settings=new Node();dashboard.id='tab-dashboard';settings.id='tab-settings';tabs.append(dashboard,module,settings);
 const nav=ui.serviceNavigation(tabs);nav.trigger.focus();nav.trigger.events.click({detail:1,preventDefault(){}});
 assert.equal(nav.menu.hidden,false);assert.equal(document.activeElement,nav.trigger);
 nav.trigger.events.keydown({key:'ArrowDown',preventDefault(){}});assert.equal(document.activeElement,module);
 nav.trigger.focus();nav.trigger.events.keydown({key:'Escape',preventDefault(){},stopPropagation(){}});assert.equal(nav.menu.hidden,true);
});

 test('Services sorts display names and keyboard navigation follows that order',()=>{
 setup();const tabs=new Node(),dashboard=new Node(),settings=new Node();dashboard.id='tab-dashboard';settings.id='tab-settings';
 const names=['Remote HTTP','Ports over SSH','Ports over Relay','Local Ports'];
 const modules=names.map(name=>{const n=new Node();n.dataset.navigationLabel=name;n.textContent=name+'Experimental';return n;});
 tabs.append(dashboard,...modules,settings);const nav=ui.serviceNavigation(tabs);
 assert.deepEqual(nav.menu.children.map(n=>n.dataset.navigationLabel),['Local Ports','Ports over Relay','Ports over SSH','Remote HTTP']);
 nav.trigger.events.keydown({key:'ArrowDown',preventDefault(){}});assert.equal(document.activeElement,modules[3]);
 nav.menu.events.keydown({key:'ArrowDown',preventDefault(){}});assert.equal(document.activeElement,modules[2]);
 assert.equal(tabs.children[0],dashboard);assert.equal(tabs.children.at(-1),settings);nav.dispose();
});

test('primary navigation is Dashboard, the Services menu of source pages, and Settings',()=>{
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 assert.match(app,/tab\('dashboard','Dashboard'\);/);assert.match(app,/tabButton=tab\(descriptor\.tab_id,descriptor\.name\)/);assert.match(app,/tab\('settings','Settings'\)/);
 assert.doesNotMatch(app,/tab\('sources'/,'there is no separate Sources page');
 const html=readFileSync(new URL('../internal/web/assets/index.html',import.meta.url),'utf8');
 assert.match(html,/<h1 class="page-title">Dashboard<\/h1>/);
 assert.doesNotMatch(html,/id="panel-dashboard"[^>]*tabindex/,'page regions are not focus targets');assert.doesNotMatch(app,/settings\.tabIndex=0/);
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8');assert.match(css,/#panels>section\[data-module\],#panel-settings>\.settings-accordion\{max-width:720px\}/,'source pages and Settings share one width');assert.doesNotMatch(html,/panel-sources/);
});

test('layout tokens define one rhythm for header, page titles, rows and surfaces',()=>{
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8').replace(/\s+/g,'');
 for(const token of ['--header-height:72px','--page-heading-height:104px','--page-title:28px','--row-height:56px','--head-row-height:40px','--gap:24px','--radius-panel:12px','--radius-control:8px'])assert.ok(css.includes(token),token);
 assert.match(css,/\.dashboard-table\{table-layout:fixed/);assert.match(css,/\.dashboard-tabletd\{height:var\(--row-height\)[^}]*white-space:nowrap/);
 assert.doesNotMatch(css,/\.source-card|\.module-view-nav|\.module-breadcrumb/,'retired page styles are removed');
 assert.match(css,/\.address-adornment>input\{text-align:right/,'the address name is right-aligned against its suffix');
 assert.match(css,/\.service-fields-fill>\.choice-list\{flex:10176px/,'a source list fills the drawer height');
 assert.match(css,/\.service-edit-body\{display:flex;flex-direction:column;gap:var\(--space-6\)\}/,'the drawer body has one vertical rhythm');
 assert.match(css,/dialog\.service-drawer\{position:fixed;inset:000auto;margin:0;width:min\(520px,100vw\)[^}]*height:100dvh/,'service forms are a full-height drawer');
});
