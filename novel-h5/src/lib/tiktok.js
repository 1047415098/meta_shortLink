const PIXEL_CODE=/^[A-Za-z0-9_-]{5,64}$/;
const EVENT_ID=/^[^\s\r\n]{1,160}$/;

function installQueue(scope,documentRef){
  if(scope.ttq)return scope.ttq;
  const queue=[];
  scope.TiktokAnalyticsObject="ttq";
  // Keep TikTok's official queue metadata: events.js reads these fields to bind
  // the downloaded SDK to the selected Pixel before it replays queued events.
  queue.methods=["page","track","identify","instances","debug","on","off","once","ready","alias","group","enableCookie","disableCookie"];
  queue.setAndDefer=(target,method)=>{target[method]=(...args)=>target.push([method,...args]);};
  for(const method of queue.methods)queue.setAndDefer(queue,method);
  queue.instance=(pixel)=>{
    const instance=queue._i?.[pixel]||[];
    for(const method of queue.methods)queue.setAndDefer(instance,method);
    return instance;
  };
  queue.load=(pixel,options={},configureScript)=>{
    const base="https://analytics.tiktok.com/i18n/pixel/events.js";
    queue._i=queue._i||{};
    queue._i[pixel]=[];
    queue._i[pixel]._u=base;
    queue._t=queue._t||{};
    queue._t[pixel]=Date.now();
    queue._o=queue._o||{};
    queue._o[pixel]=options;
    const script=documentRef.createElement("script");
    script.type="text/javascript";
    script.async=true;
    script.src=`${base}?sdkid=${encodeURIComponent(pixel)}&lib=ttq`;
    if(typeof configureScript==="function")configureScript(script);
    documentRef.head.appendChild(script);
    return script;
  };
  scope.ttq=queue;
  return queue;
}

export function installTikTokPixel({pixelCode,scope=window,documentRef=document}={}){
  const pixel=String(pixelCode||"").trim();
  if(!PIXEL_CODE.test(pixel))return false;
  const state=documentRef.documentElement?.dataset||{};
  if(state.novelTikTokPixel===pixel)return state.novelTikTokLoadFailed!=="true";
  if(state.novelTikTokPixel&&state.novelTikTokPixel!==pixel)return false;
  try{
    const ttq=installQueue(scope,documentRef);
    ttq.load(pixel,{},(script)=>{
      script.id="novel-tiktok-pixel";
      script.onerror=()=>{state.novelTikTokLoadFailed="true";};
    });
    ttq.page();
    state.novelTikTokPixel=pixel;
    return true;
  }catch{
    state.novelTikTokLoadFailed="true";
    return false;
  }
}

export function trackTikTokEvent({name,eventId,scope=window,documentRef=document}={}){
  const event=String(name||"").trim(),id=String(eventId||"").trim();
  const state=documentRef.documentElement?.dataset||{};
  const key=`novelTikTok${event}`;
  if(!/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(event)||!EVENT_ID.test(id)||state.novelTikTokLoadFailed==="true"||typeof scope.ttq?.track!=="function"||state[key]===id)return false;
  try{
    scope.ttq.track(event,{content_type:"product",contents:[]},{event_id:id});
    state[key]=id;
    return true;
  }catch{return false;}
}

export function readTikTokTTP(documentRef=document){
  const pair=String(documentRef.cookie||"").split(";").map((item)=>item.trim()).find((item)=>item.startsWith("_ttp="));
  if(!pair)return "";
  try{
    const value=decodeURIComponent(pair.slice(5)).trim();
    const unresolvedMacro=(value.length>4&&value.startsWith("__")&&value.endsWith("__"))||value.includes("{{")||value.includes("}}");
    if(!value||value.length>512||/[\r\n]/.test(value)||unresolvedMacro)return "";
    return value;
  }catch{return "";}
}
