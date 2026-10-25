import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createNavigation} from '../internal/web/assets/navigation.js';

test('primary navigation is three fixed destinations without a disclosure menu',()=>{
 const tabs={setAttribute(name,value){this[name]=value;}},visits=[];
 const nav=createNavigation({tabs,onNavigate:id=>visits.push(id)});
 assert.equal(tabs.role,'navigation');assert.equal(tabs.className,'primary-navigation');
 nav.navigate('sources');assert.deepEqual(visits,['sources']);nav.sync('local');assert.equal(nav.active(),'local');
 const app=readFileSync(new URL('../internal/web/assets/app.js',import.meta.url),'utf8');
 assert.match(app,/tab\('dashboard','Services'\);tab\('sources','Sources'\)/);assert.match(app,/tab\('settings','Settings'\)/);
 assert.doesNotMatch(app,/tab\(descriptor\.tab_id/,'source modules are reached from Sources, not the navigation bar');
});

test('layout tokens define one rhythm for header, page titles, rows and surfaces',()=>{
 const css=readFileSync(new URL('../internal/web/assets/app.css',import.meta.url),'utf8').replace(/\s+/g,'');
 for(const token of ['--header-height:72px','--page-heading-height:104px','--page-title:28px','--row-height:56px','--head-row-height:40px','--gap:24px','--radius-panel:12px','--radius-control:8px'])assert.ok(css.includes(token),token);
 assert.match(css,/\.dashboard-table\{table-layout:fixed/);assert.match(css,/\.dashboard-tabletd\{height:var\(--row-height\)[^}]*white-space:nowrap/);
 assert.match(css,/\.source-configure\{margin-top:auto;align-self:flex-start\}/);
 assert.doesNotMatch(css,/\.services-menu|\.module-view-nav/,'retired navigation styles are removed');
});
