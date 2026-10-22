import test from 'node:test';
import assert from 'node:assert/strict';
import * as ui from '../internal/web/assets/ui.js';

class Node {
 constructor(){this.children=[];this.dataset={};this.attrs={};this.events={};this.hidden=false;this.width=0;this.style={};}
 append(...nodes){for(const n of nodes){if(n.parentElement)n.parentElement.children=n.parentElement.children.filter(x=>x!==n);this.children.push(n);n.parentElement=this;}}
 prepend(n){if(n.parentElement)n.parentElement.children=n.parentElement.children.filter(x=>x!==n);this.children.unshift(n);n.parentElement=this;}
 setAttribute(k,v){this.attrs[k]=v;}
 getAttribute(k){return this.attrs[k];}
 removeAttribute(k){delete this.attrs[k];}
 addEventListener(k,v){this.events[k]=v;}
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
