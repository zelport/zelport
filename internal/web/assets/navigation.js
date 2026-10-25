// The primary navigation is three fixed destinations: Services, Sources and Settings.
export function createNavigation({tabs,onNavigate}={}){
 let current='dashboard';
 tabs.setAttribute('role','navigation');tabs.className='primary-navigation';
 return {navigate(id){onNavigate(id);},active(){return current;},sync(id){current=id;},dispose(){}};
}
