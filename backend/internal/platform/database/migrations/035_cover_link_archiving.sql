-- 封面链接删除采用归档，保留链接主记录及其历史统计关联。
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS archived_at timestamptz;

-- 管理列表只读取未归档链接，部分索引避免历史数据增长拖慢查询。
CREATE INDEX IF NOT EXISTS short_links_cover_active
ON short_links(id DESC) WHERE product_type='cover' AND archived_at IS NULL;
