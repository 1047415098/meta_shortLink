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

export async function reportReadingTime({ code,ticket,seconds,ttp,request=fetch,navigatorRef=globalThis.navigator,useBeacon=false }={}) {
  const empty={ok:false,visibleSeconds:0,tiktokEvent:null};
  if(!ticket||!Number.isInteger(seconds)||seconds<1)return empty;
  const values={ticket,seconds:String(seconds)};if(ttp)values._ttp=ttp;
  const url=`/novel/${encodeURIComponent(code)}/reading-time`,body=new URLSearchParams(values).toString();
  // pagehide uses Beacon when available; periodic reports use keepalive fetch.
  if(useBeacon&&typeof navigatorRef?.sendBeacon==="function")return { ...empty,ok:navigatorRef.sendBeacon(url,new Blob([body],{type:"application/x-www-form-urlencoded"})) };
  try{
    const response=await request(url,{method:"POST",headers:{"Content-Type":"application/x-www-form-urlencoded"},body,keepalive:true});
    if(response?.ok===false||typeof response?.json!=="function")return empty;
    const data=await response.json();
    return {ok:true,visibleSeconds:Number(data?.visible_seconds)||0,tiktokEvent:data?.tiktok_event||null};
  }catch{return empty;}
}

export async function reportStartReading({code,ticket,novelId,chapterId,chapter,ttp,request=fetch,state=globalThis.document?.documentElement?.dataset||{},retryDelays=[1000,3000],wait=(milliseconds)=>new Promise((resolve)=>setTimeout(resolve,milliseconds))}={}){
  const empty={ok:false,tiktokEvent:null};
  const novelIdentity=Number(novelId),chapterIdentity=Number(chapterId),chapterNumber=Number(chapter);
  // 必须提交当前响应中的实体 ID，避免只凭章节号产生歧义。
  if(!ticket||!Number.isInteger(novelIdentity)||novelIdentity<1||!Number.isInteger(chapterIdentity)||chapterIdentity<1||!Number.isInteger(chapterNumber)||chapterNumber<1||["pending","true"].includes(state.novelStartReadingSent))return empty;
  // pending 同时拦截切章/切语言产生的并发请求；瞬时失败最多重试两次。
  state.novelStartReadingSent="pending";
  const values={ticket,novel_id:String(novelIdentity),chapter_id:String(chapterIdentity),chapter:String(chapterNumber)};if(ttp)values._ttp=ttp;
  for(let attempt=0;attempt<=retryDelays.length;attempt+=1){
    try{
      const response=await request(`/novel/${encodeURIComponent(code)}/start-reading`,{method:"POST",headers:{"Content-Type":"application/x-www-form-urlencoded"},body:new URLSearchParams(values).toString(),keepalive:true});
      if(response?.ok!==false&&typeof response?.json==="function"){
        const data=await response.json();
        state.novelStartReadingSent="true";
        return {ok:true,tiktokEvent:data?.tiktok_event||null};
      }
      // 参数或票据被服务器拒绝时重试没有意义；只重试网络和 5xx 故障。
      if(Number(response?.status)>=400&&Number(response?.status)<500)break;
    }catch{
      // 网络异常走下一次有限重试，最终失败时释放文档标记。
    }
    if(attempt<retryDelays.length)await wait(retryDelays[attempt]);
  }
  delete state.novelStartReadingSent;
  return empty;
}
