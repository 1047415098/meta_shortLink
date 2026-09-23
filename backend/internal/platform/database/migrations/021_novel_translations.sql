-- 英文标题、简介或章节发生变化时递增，翻译任务据此拒绝发布过期结果。
ALTER TABLE novels
  ADD COLUMN source_revision bigint NOT NULL DEFAULT 1 CHECK (source_revision > 0);

CREATE TABLE novel_translation_versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  novel_id bigint NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
  locale text NOT NULL CHECK (locale IN ('id', 'ja', 'ko', 'ms', 'pt', 'fil', 'th', 'vi')),
  source_revision bigint NOT NULL CHECK (source_revision > 0),
  title text NOT NULL DEFAULT '',
  author text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT '',
  excerpt text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'failed', 'published', 'superseded')),
  enabled boolean NOT NULL DEFAULT true,
  total_items integer NOT NULL DEFAULT 0 CHECK (total_items >= 0),
  completed_items integer NOT NULL DEFAULT 0 CHECK (completed_items >= 0 AND completed_items <= total_items),
  error_message text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  published_at timestamptz
);

-- 同一种语言最多只有一个可见版本和一个正在生成的版本。
CREATE UNIQUE INDEX novel_translation_one_published
  ON novel_translation_versions(novel_id, locale) WHERE status = 'published';
CREATE UNIQUE INDEX novel_translation_one_active
  ON novel_translation_versions(novel_id, locale) WHERE status IN ('queued', 'running');
CREATE INDEX novel_translation_queue
  ON novel_translation_versions(status, created_at, id) WHERE status IN ('queued', 'running');

CREATE TABLE novel_chapter_translations (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  version_id bigint NOT NULL REFERENCES novel_translation_versions(id) ON DELETE CASCADE,
  chapter_id bigint NOT NULL REFERENCES novel_chapters(id) ON DELETE CASCADE,
  chapter_number integer NOT NULL CHECK (chapter_number > 0),
  title text NOT NULL DEFAULT '',
  body_markdown text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(version_id, chapter_id)
);

CREATE INDEX novel_chapter_translation_order
  ON novel_chapter_translations(version_id, chapter_number);

