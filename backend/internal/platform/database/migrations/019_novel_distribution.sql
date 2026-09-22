-- 旧链接保持 legacy，从而继续兼容短链接、语音小说和免费小说入口。
ALTER TABLE short_links
  ADD COLUMN IF NOT EXISTS product_type text NOT NULL DEFAULT 'legacy',
  ADD COLUMN IF NOT EXISTS novel_id bigint REFERENCES novels(id),
  ADD COLUMN IF NOT EXISTS first_visited_at timestamptz;

ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_product_type_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_product_type_check
  CHECK (product_type IN ('legacy', 'short_link', 'audio_novel', 'novel'));
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS short_links_novel_binding_check;
ALTER TABLE short_links ADD CONSTRAINT short_links_novel_binding_check
  CHECK ((product_type = 'novel' AND novel_id IS NOT NULL) OR (product_type <> 'novel' AND novel_id IS NULL));

-- 每次访问冻结入口小说，并保存浏览器累计上报的前台可见秒数。
ALTER TABLE click_events
  ADD COLUMN IF NOT EXISTS novel_id bigint REFERENCES novels(id),
  ADD COLUMN IF NOT EXISTS visible_seconds integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS visible_updated_at timestamptz;

ALTER TABLE click_events DROP CONSTRAINT IF EXISTS click_events_visible_seconds_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_visible_seconds_check
  CHECK (visible_seconds BETWEEN 0 AND 7200);

-- 明细到期清理后仍保留“曾被访问”的业务事实，防止链接重新绑定或删除。
UPDATE short_links l SET first_visited_at=history.first_visited_at
FROM (SELECT link_id,min(occurred_at) AS first_visited_at FROM click_events GROUP BY link_id) history
WHERE l.id=history.link_id AND l.first_visited_at IS NULL;

CREATE INDEX IF NOT EXISTS short_links_product_novel
  ON short_links(product_type, novel_id, id DESC);
CREATE INDEX IF NOT EXISTS clicks_novel_link_time
  ON click_events(novel_id, link_id, occurred_at DESC)
  WHERE surface = 'novel';
