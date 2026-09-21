CREATE TABLE IF NOT EXISTS novels (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title text NOT NULL,
  slug text NOT NULL,
  category text NOT NULL,
  excerpt text NOT NULL,
  body_markdown text NOT NULL,
  cover_path text NOT NULL DEFAULT '',
  published_at date NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  featured boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS novels_active_slug_unique ON novels(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS novels_one_effective_featured ON novels(featured) WHERE featured AND enabled AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS novels_public_order ON novels(published_at DESC, id DESC) WHERE enabled AND deleted_at IS NULL;

-- 初始内容让升级后的文学站可以立即展示，后续可全部在后台修改或删除。
INSERT INTO novels(title, slug, category, excerpt, body_markdown, cover_path, published_at, enabled, featured)
SELECT seed.title, seed.slug, seed.category, seed.excerpt, seed.body, seed.cover, seed.published_at::date, true, seed.featured
FROM (VALUES
  ('The Glass Orchard', 'the-glass-orchard', 'High Fantasy', 'In a valley where every promise grows as glass fruit, a mapmaker must harvest the oath she once broke.', E'# The Glass Orchard\n\nAt dawn, the orchard chimed in the mountain wind. Mara followed the sound between silver branches, carrying the map that had brought her home.\n\n---\n\nEvery fruit held a promise. Hers was waiting at the oldest tree.', '', '2026-09-20', true),
  ('A Bell for the Drowned City', 'a-bell-for-the-drowned-city', 'Mythic Fantasy', 'A bell keeper descends beneath the tide to wake a city that remembers every visitor.', E'# A Bell for the Drowned City\n\nThe sea withdrew only once each autumn. Ilyen had six hours to reach the drowned bell tower and ring it before the water returned.', '', '2026-09-18', false),
  ('The Cartographer of Ash', 'the-cartographer-of-ash', 'Adventure', 'An imperial cartographer discovers that burned kingdoms leave roads visible only at night.', E'# The Cartographer of Ash\n\nBy daylight the plain was empty. Beneath the moon, roads of pale ash crossed it like veins.', '', '2026-09-15', false),
  ('Where the Wolves Keep Winter', 'where-the-wolves-keep-winter', 'Folklore', 'To save her village, a shepherd follows a white wolf into the season hidden behind the northern hills.', E'# Where the Wolves Keep Winter\n\nWinter had failed to arrive, and the river smelled of summer rot. Then the white wolf appeared at Elra’s gate.', '', '2026-09-12', false),
  ('The Last Library of Crows', 'the-last-library-of-crows', 'Dark Fantasy', 'Each crow carries one forgotten sentence, and one librarian intends to rebuild the book they escaped.', E'# The Last Library of Crows\n\nThe first crow spoke at noon. Its voice was a page turning in an empty room.', '', '2026-09-09', false),
  ('A Kingdom Sewn in Blue', 'a-kingdom-sewn-in-blue', 'Court Fantasy', 'A royal tailor finds a rebellion stitched into the coronation cloak.', E'# A Kingdom Sewn in Blue\n\nNeris found the first secret beneath the collar: three blue stitches where the pattern demanded gold.', '', '2026-09-06', false),
  ('The Lantern at World’s End', 'the-lantern-at-worlds-end', 'Quest Fantasy', 'Two pilgrims carry the last flame beyond the edge of every known map.', E'# The Lantern at World’s End\n\nThe road ended, but the lantern’s shadow continued east across the clouds.', '', '2026-09-03', false),
  ('Salt for the Sleeping God', 'salt-for-the-sleeping-god', 'Epic Fantasy', 'A salt merchant bargains with a buried god whose dreams are changing the coastline.', E'# Salt for the Sleeping God\n\nEvery morning the coast had moved another mile inland. Sava measured it in abandoned wells.', '', '2026-09-01', false)
) AS seed(title, slug, category, excerpt, body, cover, published_at, featured)
WHERE NOT EXISTS (SELECT 1 FROM novels WHERE deleted_at IS NULL);
