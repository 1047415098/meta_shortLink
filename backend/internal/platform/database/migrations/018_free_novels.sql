-- 免费小说与语音小说保持独立，避免两个内容产品共享发布状态和正文。
CREATE TABLE novels (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title text NOT NULL,
  slug text NOT NULL,
  author text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT '',
  excerpt text NOT NULL,
  cover_path text NOT NULL DEFAULT '',
  published_at date NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  featured boolean NOT NULL DEFAULT false,
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX novels_active_slug_unique ON novels(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX novels_one_effective_featured ON novels(featured) WHERE featured AND enabled AND deleted_at IS NULL;
CREATE INDEX novels_public_order ON novels(sort_order DESC, published_at DESC, id DESC) WHERE enabled AND deleted_at IS NULL;

CREATE TABLE novel_chapters (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  novel_id bigint NOT NULL REFERENCES novels(id),
  chapter_number integer NOT NULL CHECK (chapter_number > 0),
  title text NOT NULL DEFAULT '',
  body_markdown text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX novel_chapters_active_number_unique ON novel_chapters(novel_id, chapter_number) WHERE deleted_at IS NULL;
CREATE INDEX novel_chapters_public_order ON novel_chapters(novel_id, chapter_number) WHERE enabled AND deleted_at IS NULL;

ALTER TABLE click_events DROP CONSTRAINT click_events_surface_check;
ALTER TABLE click_events ADD CONSTRAINT click_events_surface_check
  CHECK (surface IN ('short_link', 'audio_novel', 'novel'));
