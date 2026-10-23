import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

test('component gallery preview includes every shared UI module import',()=>{
 const preview=readFileSync(new URL('./ui/preview.py',import.meta.url),'utf8');
 const facade=readFileSync(new URL('../internal/web/assets/ui.js',import.meta.url),'utf8');
 const imports=[...facade.matchAll(/from ['"]\.\/(ui-[^'"]+\.js)['"]/g)].map(match=>match[1]);
 assert.ok(imports.length>0,'the shared UI facade should expose module imports');
 for(const file of imports)assert.match(preview,new RegExp(`internal/web/assets/${file.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}`));
});
