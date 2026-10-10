// 封面墙的结果页只回看问答和抽奖福利，不自动进入小说。
export const COVER_WALL_STAGES = Object.freeze({
  INTRO: "intro",
  QUESTIONS: "questions",
  REVEAL: "reveal",
  CONVERSION: "conversion",
});

// 问答只控制展示顺序，不更改短链的广告归因绑定。
export function nextCoverWallStage(stage) {
  const stages = {
    [COVER_WALL_STAGES.INTRO]: COVER_WALL_STAGES.QUESTIONS,
    [COVER_WALL_STAGES.QUESTIONS]: COVER_WALL_STAGES.REVEAL,
    [COVER_WALL_STAGES.REVEAL]: COVER_WALL_STAGES.CONVERSION,
  };
  return stages[stage] || null;
}

// 在 280–359 中抽取展示序号，并排除上一次结果，保证连续刷新不会出现相同数字。
export function nextLuckyUserNumber(previous, random = Math.random) {
  const minimum = 280;
  const count = 80;
  const last = Number(previous);
  const hasPrevious = Number.isInteger(last) && last >= minimum && last < minimum + count;
  const index = Math.floor(random() * (count - Number(hasPrevious)));
  return minimum + index + (hasPrevious && index >= last - minimum ? 1 : 0);
}
