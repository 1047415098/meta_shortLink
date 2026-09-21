-- 演示正文由详情页统一显示作品标题，移除种子内容中重复的首行标题。
UPDATE novels
SET body_markdown = regexp_replace(body_markdown, '^# [^\n]+\n\n', ''),
    updated_at = now()
WHERE slug IN (
  'the-glass-orchard',
  'a-bell-for-the-drowned-city',
  'the-cartographer-of-ash',
  'where-the-wolves-keep-winter',
  'the-last-library-of-crows',
  'a-kingdom-sewn-in-blue',
  'the-lantern-at-worlds-end',
  'salt-for-the-sleeping-god'
)
AND body_markdown LIKE '# %';
