import {el} from './ui-dom.js';

// A source page shows its configuration. Status or catalog content a module keeps
// beside it is placed before or after; service lists live only on Services.
export function moduleViews(ctx,{overview=el('section'),configuration=el('section'),onChange,overviewPlacement='hidden'}={}){
 const actions=el('div','module-view-actions');
 let configured=false,disposed=false;
 const toolbar=ctx.toolbar||el('div','module-toolbar module-inline-toolbar');
 toolbar.append(actions);if(!ctx.toolbar)ctx.panel.append(toolbar);
 if(overviewPlacement==='before')ctx.panel.append(overview,configuration);else ctx.panel.append(configuration,overview);
 overview.hidden=overviewPlacement==='hidden';actions.hidden=overview.hidden;configuration.hidden=false;
 // Configuration is always visible; the first request lets modules build it lazily.
 function show(view){if(disposed||view!=='configuration'||configured)return;configured=true;onChange?.(view);}
 return {overview,configuration,actions,show,dispose(){disposed=true;actions.remove();overview.remove();configuration.remove();if(!ctx.toolbar)toolbar.remove();}};
}
