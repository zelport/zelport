import {serviceNavigation} from './ui.js';

export function createNavigation({tabs,onNavigate}={}){
 let current='dashboard';
 const services=serviceNavigation(tabs);
 return {
  navigate(id){onNavigate(id);},
  active(){return current;},
  sync(id){current=id;services.select();},
  dispose(){services.dispose();}
 };
}
