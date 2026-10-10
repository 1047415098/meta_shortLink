export function installMetaPixel({ pixelId, eventId, scope = window, documentRef = document } = {}) {
  const pixel=String(pixelId||"").trim(); if(!/^\d{5,30}$/.test(pixel)) return false;
  if(!scope.fbq){ const fbq=function(){fbq.callMethod?fbq.callMethod.apply(fbq,arguments):fbq.queue.push(arguments);};Object.assign(fbq,{push:fbq,loaded:true,version:"2.0",queue:[]});scope.fbq=fbq;scope._fbq ||= fbq;const script=documentRef.createElement("script");script.id="novel-meta-pixel";script.async=true;script.src="https://connect.facebook.net/en_US/fbevents.js";documentRef.head.appendChild(script); }
  const state=documentRef.documentElement.dataset;
  if(state.novelMetaPixel!==pixel){scope.fbq("init",pixel);state.novelMetaPixel=pixel;}
  if(eventId&&state.novelMetaPageView!==eventId){scope.fbq("track","PageView",{},{eventID:eventId});state.novelMetaPageView=eventId;}
  return true;
}
export function trackMetaTimeSpent(eventId, scope=window, documentRef=document){const event=String(eventId||""),state=documentRef.documentElement.dataset;if(!event||typeof scope.fbq!=="function"||state.novelMetaTimeSpent===event)return false;state.novelMetaTimeSpent=event;scope.fbq("trackCustom","TimeSpent",{},{eventID:event});return true;}
