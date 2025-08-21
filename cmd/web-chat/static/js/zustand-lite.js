// Minimal zustand-like store (no deps)
// API: const useStore = createStore(setupFn); useStore.get(), useStore.set(partial), useStore.subscribe(cb)
(function(global){
  function createStore(init){
    let state = {};
    const subs = new Set();
    const set = (partial)=>{
      const next = typeof partial === 'function' ? partial(state) : partial;
      state = Object.assign({}, state, next);
      subs.forEach(cb=>{ try { cb(state); } catch(_){} });
    };
    const get = ()=>state;
    const subscribe = (cb)=>{ subs.add(cb); return ()=>subs.delete(cb); };
    state = init(set, get);
    return { get, set, subscribe };
  }
  global.createStore = createStore;
})(window);


