function positiveChapterNumber(value) {
  const chapter=Number(value);
  return Number.isInteger(chapter)&&chapter>0?chapter:null;
}

export function entryRouteForBootstrap(link,currentRoute) {
  if(!link?.code||!link.entry_story_slug||currentRoute?.name!=="home")return null;
  const params={code:link.code,slug:link.entry_story_slug};
  const query={...(currentRoute.query||{})};
  const chapter=positiveChapterNumber(link.entry_chapter_number);
  // 新投放链直达绑定章节；没有章节的历史链接继续进入小说简介页。
  return chapter
    ?{name:"reader",params:{...params,chapter},query}
    :{name:"story",params,query};
}

export function shouldReportStartReading({entryChapterNumber,currentChapterNumber,firstReadableChapterNumber}) {
  const current=positiveChapterNumber(currentChapterNumber);
  const entry=positiveChapterNumber(entryChapterNumber);
  // 历史链接没有入口章节，仍以该小说的第一条可读章节作为开始阅读点。
  const target=entry||positiveChapterNumber(firstReadableChapterNumber)||1;
  return current===target;
}
