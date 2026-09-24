import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { installMetaPixel } from "../src/lib/meta.js";
import { installTikTokPixel, readTikTokTTP, trackTikTokEvent } from "../src/lib/tiktok.js";

function fakeDocument(cookie="") {
  const appended=[],dataset={};
  const documentRef={
    cookie,
    documentElement:{ dataset },
    head:{ appendChild(node){appended.push(node);} },
    createElement:()=>({}),
  };
  return { appended,dataset,documentRef };
}

test("TikTok pixel initializes once and shares the server event id", () => {
  const scope={}, {appended,dataset,documentRef}=fakeDocument();
  assert.equal(installTikTokPixel({pixelCode:"C0ABC123",scope,documentRef}),true);
  installTikTokPixel({pixelCode:"C0ABC123",scope,documentRef});
  assert.equal(trackTikTokEvent({name:"ViewContent",eventId:"novel_v1_qualified",scope,documentRef}),true);
  assert.equal(trackTikTokEvent({name:"ViewContent",eventId:"novel_v1_qualified",scope,documentRef}),false);
  assert.equal(appended.filter((node)=>node.id==="novel-tiktok-pixel").length,1);
  assert.equal(appended[0].src,"https://analytics.tiktok.com/i18n/pixel/events.js?sdkid=C0ABC123&lib=ttq");
  assert.equal(scope.TiktokAnalyticsObject,"ttq");
  assert.equal(scope.ttq._i.C0ABC123._u,"https://analytics.tiktok.com/i18n/pixel/events.js");
  assert.equal(typeof scope.ttq.instance,"function");
  assert.equal(dataset.novelTikTokViewContent,"novel_v1_qualified");
  assert.equal(scope.ttq.filter((item)=>item[0]==="page").length,1);
  const tracked=scope.ttq.find((item)=>item[0]==="track");
  assert.equal(tracked[1],"ViewContent");
  assert.equal(tracked[3].event_id,"novel_v1_qualified");
});

test("TikTok loader rejects unsafe inputs and fails closed after SDK error", () => {
  const scope={}, {appended,documentRef}=fakeDocument();
  assert.equal(installTikTokPixel({pixelCode:"bad code",scope,documentRef}),false);
  assert.equal(appended.length,0);
  assert.equal(installTikTokPixel({pixelCode:"C0ABC123",scope,documentRef}),true);
  assert.doesNotThrow(()=>appended[0].onerror());
  assert.equal(trackTikTokEvent({name:"StartReading",eventId:"event-1",scope,documentRef}),false);
  assert.equal(trackTikTokEvent({name:"StartReading",eventId:"",scope,documentRef}),false);
});

test("TikTok _ttp is read from document Cookie without exposing other Cookies", () => {
  const {documentRef}=fakeDocument("session=private; _ttp=ttp%2Dcookie%2D123; theme=dark");
  assert.equal(readTikTokTTP(documentRef),"ttp-cookie-123");
  documentRef.cookie="_ttp=cookie__segment";
  assert.equal(readTikTokTTP(documentRef),"cookie__segment");
  documentRef.cookie="_ttp=bad%0Avalue";
  assert.equal(readTikTokTTP(documentRef),"");
});

test("App selects exactly one browser advertising platform", async () => {
  const source=await readFile(new URL("../src/App.vue",import.meta.url),"utf8");
  assert.match(source,/bootstrap\.ad_platform\s*===\s*"meta"/);
  assert.match(source,/bootstrap\.ad_platform\s*===\s*"tiktok"/);

  const meta=fakeDocument(),tikTok=fakeDocument();
  assert.equal(installTikTokPixel({pixelCode:"",scope:{},documentRef:meta.documentRef}),false);
  assert.equal(meta.appended.length,0,"Meta bootstrap must not append TikTok SDK");
  assert.equal(installMetaPixel({pixelId:"",eventId:"",scope:{},documentRef:tikTok.documentRef}),false);
  assert.equal(tikTok.appended.length,0,"TikTok bootstrap must not append Meta SDK");
});
