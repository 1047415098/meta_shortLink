import test from "node:test";
import assert from "node:assert/strict";
import { bannerStories, homeSections, normalizedQuery } from "../src/lib/content.js";
test("home sections map empty arrays safely", () => assert.deepEqual(homeSections({}), { featured:null, ranking:[], items:[] }));
test("search query trimming does not request empty input", () => { assert.equal(normalizedQuery("  red moon "), "red moon"); assert.equal(normalizedQuery("   "), ""); });
test("home banner keeps featured first and removes duplicate stories", () => {
  const featured={id:2,slug:"two"},items=[{id:1,slug:"one"},{id:2,slug:"two"},{id:3,slug:"three"}];
  assert.deepEqual(bannerStories(featured,items,2).map((item)=>item.id),[2,1]);
  assert.deepEqual(bannerStories(null,[],5),[]);
});
