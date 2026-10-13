-- Brand logos, descriptions and "read more" links for Be Lenka + Barebarics
-- (+ Freet's read-more link)
-- (source texts: data/{brand}/about.txt). Idempotent (in cmd/dbimport's
-- always-run list) and ADD-only for the schema, so safe against deployed code.
-- Descriptions use DO UPDATE: this file is their source — edit here, re-run dbimport.
--
-- DEPLOY ORDER (migration-first): the brands page reads brands.about_url, so run
-- `go run ./cmd/dbimport` on prod BEFORE merging the code.

-- Optional link to the brand's own "about" page, shown as "Read more ↗" on its card.
ALTER TABLE brands ADD COLUMN IF NOT EXISTS about_url TEXT;

-- Be Lenka is normally created by cmd/addbrands; ensure it exists (hidden — its
-- catalog.json reveals it) so a fresh DB can import its products too.
INSERT INTO brands (name, sku, slug, is_featured, sort_order, is_visible)
VALUES ('Be Lenka', 'be-lenka', 'be-lenka', FALSE, 9, FALSE)
ON CONFLICT (slug) DO NOTHING;

UPDATE brands SET
  logo_url = '/images/be-lenka/be_lenka-logo-white.svg',
  country_of_origin = 'SK',
  year_founded = 2017,
  website_url = 'https://www.belenka.com',
  about_url = 'https://www.belenka.com/about-us/'
WHERE slug = 'be-lenka';

UPDATE brands SET
  logo_url = '/images/barebarics/barebarics_logo-white.svg',
  about_url = 'https://www.barebarics.com/by-be-lenka/'
WHERE slug = 'barebarics';

UPDATE brands SET about_url = 'https://freetbarefoot.com/pages/company'
WHERE slug = 'freet';

INSERT INTO brand_translations (brand_id, lang, description)
SELECT b.id, v.lang::lang_code, v.description
FROM brands b
JOIN (VALUES
  ('be-lenka', 'mk', 'Словачки бренд за босоноги обувки основан во 2017 година, денес едно од најпознатите имиња во оваа категорија, со испорака во повеќе од 80 земји. Обувки за секој ден со широк анатомски простор за прстите, флексибилни ѓонови и нула надолен пад, изработени од внимателно избрани квалитетни материјали со грижа за секој детал — за стапалото да се движи природно од утро до вечер.'),
  ('be-lenka', 'sq', 'Markë sllovake e këpucëve barefoot, e themeluar në vitin 2017 dhe sot një nga emrat më të njohur në këtë kategori, me dërgesa në më shumë se 80 vende. Këpucë për çdo ditë me hapësirë të gjerë anatomike për gishtërinjtë, shollë fleksibël dhe zero drop, të punuara nga materiale cilësore të zgjedhura me kujdes dhe me vëmendje për çdo detaj — që këmba të lëvizë natyrshëm nga mëngjesi në mbrëmje.'),
  ('be-lenka', 'en', 'Slovak barefoot shoe brand founded in 2017 and now one of the best-known names in barefoot footwear, shipped to more than 80 countries. Everyday shoes with an anatomically wide toe box, flexible soles and zero drop, made from carefully selected premium materials with attention to every detail — so your feet can move naturally from morning to evening.'),
  ('barebarics', 'mk', 'Брендот за патики на Be Lenka од Словачка — за сите што сакаат удобност на босоного одење без минималистички изглед. Смели урбани силуети, контрастни детали и посилни бои, на истата босонога основа: анатомски широк простор за прстите, флексибилен ѓон и нула надолен пад. Создадени за градот, за цел ден носење и секојдневно движење.'),
  ('barebarics', 'sq', 'Marka e atleteve e Be Lenka nga Sllovakia — për këdo që dëshiron komoditetin barefoot pa pamjen minimaliste. Silueta urbane të guximshme, detaje kontrastuese dhe ngjyra më të forta, mbi të njëjtën bazë barefoot: hapësirë anatomike e gjerë për gishtërinjtë, shollë fleksibël dhe zero drop. Të krijuara për qytetin, për veshje gjithë ditën dhe lëvizje të përditshme.'),
  ('barebarics', 'en', 'Be Lenka''s sneaker brand from Slovakia — for anyone who wants barefoot comfort without the minimalist look. Bold urban silhouettes, contrast details and stronger colours, built on the same barefoot base: an anatomically wide toe box, a flexible sole and zero drop. Made for the city, all-day wear and everyday movement.')
) AS v(slug, lang, description) ON v.slug = b.slug
ON CONFLICT (brand_id, lang) DO UPDATE SET description = EXCLUDED.description;
