export function createVisibleTimeTracker({ threshold=0, onThreshold, onTick, documentRef=document, now=()=>performance.now(), schedule=(fn)=>setInterval(fn,250), cancel=clearInterval }={}) {
  let started=documentRef.visibilityState==="visible"?now():null,elapsed=0,sent=false,lastSecond=-1;
  const emit=()=>{
    const seconds=Math.floor((elapsed+(started===null?0:now()-started))/1000),target=Number(threshold);
    // Consumers receive only second changes, avoiding duplicate periodic network reports.
    if(seconds!==lastSecond){lastSecond=seconds;onTick?.(seconds);}
    if(!sent&&Number.isInteger(target)&&target>0&&seconds>=target){sent=true;onThreshold?.();}
  };
  const changed=()=>{if(documentRef.visibilityState==="visible"){if(started===null)started=now();}else if(started!==null){elapsed+=now()-started;started=null;}emit();};
  documentRef.addEventListener("visibilitychange",changed);const timer=schedule(emit);emit();
  return()=>{cancel(timer);documentRef.removeEventListener("visibilitychange",changed);};
}

export async function reportTimeSpent({ code,ticket,request=fetch,state=document.documentElement.dataset }={}) {
  if(!ticket||state.novelTimeSpentSent==="true")return false;
  state.novelTimeSpentSent="true";
  try{await request(`/novel/${encodeURIComponent(code)}/time-spent`,{method:"POST",headers:{"Content-Type":"application/x-www-form-urlencoded"},body:new URLSearchParams({ticket}).toString(),keepalive:true});return true;}catch{return false;}
}

export async function reportReadingTime({ code,ticket,seconds,request=fetch,navigatorRef=globalThis.navigator,useBeacon=false }={}) {
  if(!ticket||!Number.isInteger(seconds)||seconds<1)return false;
  const url=`/novel/${encodeURIComponent(code)}/reading-time`,body=new URLSearchParams({ticket,seconds:String(seconds)}).toString();
  // pagehide uses Beacon when available; periodic reports use keepalive fetch.
  if(useBeacon&&typeof navigatorRef?.sendBeacon==="function")return navigatorRef.sendBeacon(url,new Blob([body],{type:"application/x-www-form-urlencoded"}));
  try{const response=await request(url,{method:"POST",headers:{"Content-Type":"application/x-www-form-urlencoded"},body,keepalive:true});return response?.ok!==false;}catch{return false;}
}
