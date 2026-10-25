import {button,el} from './ui-dom.js';

// Modules own view contents. This component owns their shared navigation layout.
export function moduleViews(ctx,{overview=el('section'),configuration=el('section'),onChange}={}){
 const actions=el('div','module-view-actions'),navigation=el('nav','module-view-nav');
 let current='overview',disposed=false;
 const overviewButton=button('Overview',()=>show('overview'),'module-view-button');
 const configurationButton=button('Configuration',()=>show('configuration'),'module-view-button');
 navigation.setAttribute('aria-label',`${ctx.moduleName||'Module'} views`);
 navigation.append(overviewButton,configurationButton);
 const toolbar=ctx.toolbar||el('div','module-toolbar module-inline-toolbar');
 toolbar.append(actions,navigation);if(!ctx.toolbar)ctx.panel.append(toolbar);
 ctx.panel.append(overview,configuration);
 function update(){
  const config=current==='configuration';overview.hidden=config;configuration.hidden=!config;actions.hidden=config;
  overviewButton.setAttribute('aria-current',config?'false':'page');configurationButton.setAttribute('aria-current',config?'page':'false');
 }
 function show(view){if(disposed||view===current)return;current=view;update();onChange?.(view);}
 update();
 return {overview,configuration,actions,navigation,show,dispose(){disposed=true;actions.remove();navigation.remove();overview.remove();configuration.remove();if(!ctx.toolbar)toolbar.remove();}};
}
