-- freet-update.sql
-- Idempotent updates to existing Freet products + 3 new products
-- (Feldom 3, Tanga 3, Pace). Safe to re-run.
--
-- Changes to existing products:
--   - Image paths updated to include color subfolders
--   - is_featured + sort_order set per the new display order
--   - Chamois set is_published=FALSE (hidden on frontend, data kept)
--   - Vibe 2 white gallery images added
--
-- New display order: Feldom 3 (1), Vibe 2 (2), Tanga 3 (3), York 2 (4),
--   Keld 3 (5), Richmond 2 (6), Pace (7), Chamois (hidden)

-- ── 1. Update existing product metadata ──────────────────────────────────────

UPDATE products SET
    is_featured  = FALSE,
    is_published = FALSE,
    sort_order   = 8,
    image_url    = '/images/freet/chamois/images/brown-black/chamois.webp'
WHERE sku = 'FREET-CHAMOIS';

UPDATE products SET
    is_featured = FALSE,
    sort_order  = 5,
    image_url   = '/images/freet/keld-3/images/olive/keld-3.webp'
WHERE sku = 'FREET-KELD-3';

UPDATE products SET
    is_featured = FALSE,
    sort_order  = 6,
    image_url   = '/images/freet/richmond-2/images/brown-black/richmond-2.webp'
WHERE sku = 'FREET-RICHMOND-2';

UPDATE products SET
    is_featured = TRUE,
    sort_order  = 2,
    image_url   = '/images/freet/vibe-2/images/black/vibe-2.webp'
WHERE sku = 'FREET-VIBE-2';

UPDATE products SET
    is_featured = TRUE,
    sort_order  = 4,
    image_url   = '/images/freet/york-2/images/black/york-2.webp'
WHERE sku = 'FREET-YORK-2';

-- ── 2. Gallery paths (already consolidated in freet.sql) ──────────────────────

-- ── 3. Vibe 2 — add white colour gallery ─────────────────────────────────────

INSERT INTO product_gallery (product_id, image_url, sort_order)
VALUES
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-1.webp', 8),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-2.webp', 9),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-3.webp', 10),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-4.webp', 11),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-5.webp', 12),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-6.webp', 13),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-7.webp', 14)
ON CONFLICT DO NOTHING;

-- ── 4. New products ───────────────────────────────────────────────────────────

INSERT INTO products (sku, name, slug, brand_id, category_id, gender_id, price_mkd, image_url, is_active, is_published, is_featured, sort_order)
VALUES
    (
        'FREET-FELDOM-3', 'Feldom 3', 'feldom-3',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        6200,
        '/images/freet/feldom-3/images/olive-green/feldom-3.webp',
        TRUE, TRUE, TRUE, 1
    ),
    (
        'FREET-TANGA-3', 'Tanga 3', 'tanga-3',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        6200,
        '/images/freet/tanga-3/images/blue/tanga-3.webp',
        TRUE, TRUE, TRUE, 3
    ),
    (
        'FREET-PACE', 'Pace', 'pace',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        6500,
        '/images/freet/pace/images/black/pace.webp',
        TRUE, TRUE, FALSE, 7
    )
ON CONFLICT (sku) DO NOTHING;

-- ── 5. Sizes + size chart for new products ────────────────────────────────────

INSERT INTO product_sizes (product_id, eu_size)
SELECT p.id, s.eu_size FROM products p
CROSS JOIN (VALUES (36),(37),(38),(39),(40),(41),(42),(43),(44),(45),(46)) AS s(eu_size)
WHERE p.sku IN ('FREET-FELDOM-3','FREET-TANGA-3','FREET-PACE')
ON CONFLICT (product_id, eu_size) DO NOTHING;

INSERT INTO size_chart (product_id, eu_size, foot_length_mm)
SELECT p.id, s.eu_size, s.len FROM products p
CROSS JOIN (VALUES
    (36, 230),(37, 237),(38, 244),(39, 251),(40, 258),
    (41, 265),(42, 272),(43, 279),(44, 286),(45, 293),(46, 300)
) AS s(eu_size, len)
WHERE p.sku IN ('FREET-FELDOM-3','FREET-TANGA-3','FREET-PACE')
ON CONFLICT (product_id, eu_size) DO NOTHING;

-- ── 6. Colours ────────────────────────────────────────────────────────────────

INSERT INTO product_colors (product_id, color, hex)
VALUES
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Olive Green', '#606b40'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'),  'Blue',        '#3a6ea5'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'),  'Salmon',      '#e8875c'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'Black',       '#1a1a1a'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'Charcoal',    '#3d3d3d')
ON CONFLICT (product_id, color) DO NOTHING;

-- ── 7. Stock (all sizes × colours at 0 — reservation mode; update on delivery) ─

INSERT INTO product_stock (product_id, size_id, color_id, qty)
SELECT p.id, ps.id, pc.id, 0
FROM products p
JOIN product_sizes ps ON ps.product_id = p.id
JOIN product_colors pc ON pc.product_id = p.id
WHERE p.sku IN ('FREET-FELDOM-3','FREET-TANGA-3','FREET-PACE')
ON CONFLICT (product_id, size_id, color_id) DO NOTHING;

-- Vibe 2 white stock entries (colour already exists in DB)
INSERT INTO product_stock (product_id, size_id, color_id, qty)
SELECT p.id, ps.id, pc.id, 0
FROM products p
JOIN product_sizes ps ON ps.product_id = p.id
JOIN product_colors pc ON pc.product_id = p.id
WHERE p.sku = 'FREET-VIBE-2' AND pc.color = 'White'
ON CONFLICT (product_id, size_id, color_id) DO NOTHING;

-- ── 8. Activities ─────────────────────────────────────────────────────────────

INSERT INTO product_activities (product_id, activity)
VALUES
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'running'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'hiking'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'),  'casual'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'),  'training'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'),  'office'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'running'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'hiking'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'training'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'),     'casual')
ON CONFLICT (product_id, activity) DO NOTHING;

-- ── 9. Gallery ────────────────────────────────────────────────────────────────

INSERT INTO product_gallery (product_id, image_url, sort_order)
VALUES
    -- Feldom 3 (Olive Green)
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), '/images/freet/feldom-3/images/olive-green/feldom-3-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), '/images/freet/feldom-3/images/olive-green/feldom-3-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), '/images/freet/feldom-3/images/olive-green/feldom-3-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), '/images/freet/feldom-3/images/olive-green/feldom-3-4.webp', 4),
    -- Tanga 3 (Blue first, then Salmon)
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/blue/tanga-3-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/blue/tanga-3-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/blue/tanga-3-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/blue/tanga-3-4.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/salmon/tanga-3-1.webp', 5),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/salmon/tanga-3-2.webp', 6),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/salmon/tanga-3-3.webp', 7),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), '/images/freet/tanga-3/images/salmon/tanga-3-4.webp', 8),
    -- Pace (Black first, then Charcoal)
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/black/pace-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/black/pace-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/black/pace-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/black/pace-4.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/black/pace-5.webp', 5),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/charcoal/pace-1.webp', 6),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/charcoal/pace-2.webp', 7),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/charcoal/pace-3.webp', 8),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/charcoal/pace-4.webp', 9),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), '/images/freet/pace/images/charcoal/pace-5.webp', 10)
ON CONFLICT (product_id, image_url) DO NOTHING;

-- ── 10. Specs ─────────────────────────────────────────────────────────────────

INSERT INTO product_specs (product_id, key, value)
VALUES
    -- Feldom 3
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Upper',        'BottleYarn breathable mesh (recycled plastic)'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Lining',       'Breathable moisture-moving lining'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Insole',       'ReBound 4mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Outsole',      'HillGrip 4.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Weight',       '522g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Stack Height', '10.5mm with insole / 7.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'Drop',         'Zero drop'),
    -- Tanga 3
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Upper',        'CoffeYarn breathable mesh (recycled coffee grounds)'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Lining',       'Unlined'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Insole',       'Flexile 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Outsole',      'UrbanGrip 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Weight',       '460g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Stack Height', '9.5mm with insole / 6.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'Drop',         'Zero drop'),
    -- Pace
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Upper',        'CoffeYarn breathable mesh (recycled coffee grounds)'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Lining',       'Unlined'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Insole',       'OrthoLite 6.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Outsole',      'MultiGrip 2.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Weight',       '452g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Stack Height', '11.5mm with insole / 5.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 'Drop',         'Zero drop')
ON CONFLICT (product_id, key) DO NOTHING;

-- ── 11. Highlights ────────────────────────────────────────────────────────────

INSERT INTO product_highlights (product_id, sort_order, mk, sq, en)
VALUES
    -- Feldom 3
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 1, 'Брзо сушење',  'Tharje e shpejtë', 'QUICK-DRY'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 2, 'Дишливо',      'I ajrosur',        'BREATHABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 3, 'Цврсто',       'I fortë',          'TOUGH'),
    ((SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 4, 'Патека грип',  'Grip shtegu',      'TRAIL GRIP'),
    -- Tanga 3
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 1, 'Дишливо',      'I ajrosur',        'BREATHABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 2, 'Брзо сушење',  'Tharje e shpejtë', 'QUICK-DRY'),
    ((SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 3, 'Лесно',        'I lehtë',          'LIGHTWEIGHT'),
    -- Pace
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 1, 'Лесно',           'I lehtë',          'LIGHTWEIGHT'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 2, 'Удобност',        'Komfort',          'COMFORT'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 3, 'Одржливо',        'I qëndrueshëm',    'SUSTAINABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-PACE'), 4, 'Урбано и патека', 'Urbane dhe shteg', 'URBAN & TRAIL')
ON CONFLICT DO NOTHING;

-- ── 12. Translations ──────────────────────────────────────────────────────────

-- Feldom 3 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'mk',
    'Грипована, издржлива перформанс патика за трчање и планинарење на широк спектар на патечки површини.',
    E'Минимална патика за трчање и планинарење погодна за патеки и надвор од нив со нов дизајн и посилна, подишлива и поудобна горна мрежа.\n\nПришиена HillGrip ѓон со подлабок шаблон од 4мм за подобар грип на помеки патеки.\n\nДури и поиздржлива со прошиено засилување на местата со голема истроеност.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nBottleYarn: дишлива, издржлива, мрежа што ја носи влагата, направена од рециклирани пластични шишиња.\nПрошиено засилување на места со голема истроеност.\nРамни врвки — зелени со лимено мрснило.\n\nОбвивка\nДишлива, мрежа која ја носи влагата.\n\nВлошка\nReBound 4 мм: енергетски одѕив со осет за подлогата за целосно природно движење. Идеално за подобри перформанси на потврди подлоги и подолги растојанија.\n\nСредна потпетица\nSoftFlex: целосно флексибилна за максимално движење на стапалото.\n\nГон\nHillGrip: 4,0 мм газен профил обезбедува добра тракција на тврди и кални патечки површини.\n\nВодоотпорност\nНе е водоотпорно, високо дишливо. Користете водоотпорни чорапи за студени/влажни услови.\n\nТежина\n522 грама на пар во големина 42.\n\nВисина на ѓонот\n10,5 мм со влошка; 7,5 мм без влошка.\n\nНега\nРачно перете со млака вода и благ детергент. Сушете на воздух.\nНе препорачуваме ставање на вашите Freet патики во машина за перење или сушара.',
    E'Горен дел: 60% рециклирано (BottleYarn)\nГон: 60% природен каучук\nОбвивка: 65% рециклирано (BottleYarn)\nВрвки: 70% рециклирано (BottleYarn)\nВлошка: 60% рециклирано\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија\nНадворешно пакување: рециклирана и рециклабилна пластика или картон/хартиен пакет'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Feldom 3 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'sq',
    'Këpucë performante me grip të mirë dhe të qëndrueshme për vrapim dhe hiking në sipërfaqe të ndryshme shtegjesh.',
    E'Këpucë minimale vrapimi dhe ecjeje malore e përshtatshme për shtigje dhe jashtë tyre me dizajn të ri dhe rrjetë të sipërme më të fortë, më të ajrosur dhe komode.\n\nSolla e jashtme HillGrip e qepur me model 4mm më të thellë për grip më të mirë në shtigje më të buta.\n\nEdhe më e qëndrueshme me mbulues të qepur në zonat e konsumimit të lartë.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nBottleYarn: rrjetë e ajrosur, e qëndrueshme, që largon lagështinë, prej shisheve plastike të ricikluara.\nMbulues i qepur në zonat e konsumimit të lartë.\nLidhëse të sheshta — të gjelbra me refleks gëlqeror.\n\nVeshja\nVeshje e ajrosur, që largon lagështinë.\n\nSole e brendshme\nReBound 4 mm: kthim energjie me ndjesi të tokës për lëvizje natyrale. Ideale për performancë në sipërfaqe më të forta dhe distanca më të gjata.\n\nMidsole\nSoftFlex: plotësisht fleksibël për lëvizje maksimale të këmbës.\n\nSolla e jashtme\nHillGrip: 4.0 mm gjurmë jep trakcion të mirë në sipërfaqe shtegjesh të forta dhe me baltë.\n\nPapërshkueshmëria nga uji\nNuk është i papërshkueshëm nga uji, shumë i ajrosur. Përdor çorape të papërshkueshme për kushte të ftohta / të lagështa.\n\nPesha\n522 gramë çift në madhësinë 42.\n\nLartësia e solës\n10.5 mm me sole; 7.5 mm pa sole.\n\nKujdesi\nLani me dorë me ujë të vakët dhe detergjent të butë. Thajeni në ajër.\nNuk rekomandojmë vendosjen e Freet-eve tuaj në lavatriçe ose tharëse.',
    E'Pjesa e sipërme: 60% e ricikluar (BottleYarn)\nSolla e jashtme: 60% gomë natyrale\nVeshja: 65% e ricikluar (BottleYarn)\nLidhëset: 70% të ricikluara (BottleYarn)\nSolla e brendshme: 60% e ricikluar\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme\nPaketimi i jashtëm: plastikë ose karton/letër i ricikluar dhe i riciklueshëm'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Feldom 3 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-FELDOM-3'), 'en',
    'Grippy, durable performance for running and hiking across a wide range of trail surfaces.',
    E'Minimal running and hiking shoe suitable for both on and off the trail with new design and stronger, more breathable and comfortable upper mesh.\n\nStitched HillGrip outsole with a deeper 4mm lug pattern for better grip on softer trails.\n\nEven more durable with stitched overlay across high wear areas.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nBottleYarn: breathable, durable, moisture moving mesh, made from recycled plastic bottles.\nStitched overlay on high impact and wear areas.\nFlat lace — green with lime fleck.\n\nLining\nBreathable, moisture moving lining.\n\nInsole\nReBound 4 mm: energy return with ground connectivity to allow full natural movement. Ideal for performance on harder surfaces and longer distances.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nHillGrip: 4.0 mm tread gives good traction on hard and muddy trail surfaces.\n\nWaterproofness\nNot waterproof, highly breathable. Use a waterproof sock for cold / wet conditions.\n\nWeight\n522 grams per size 42.\n\nStack Height\n10.5 mm with insole; 7.5 mm without insole.\n\nCare\nHand wash with luke warm water and mild detergent. Air dry.\nWe do not recommend putting your Freets in the washing machine or dryer.',
    E'Upper: 60% recycled (BottleYarn)\nOutsole: 60% natural rubber\nLining: 65% recycled (BottleYarn)\nLace: 70% recycled (BottleYarn)\nInsole: 60% recycled\nPackaging: box/wrapping recycled and recyclable card & paper\nOuter packaging: recycled and recyclable plastic or card/paper packet'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Tanga 3 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'mk',
    'Ултра-лесна дишлива патика за теретана, патување и урбана средина со флексибилна целодневна удобност.',
    E'Ултра-лесна дишлива патика за секојдневно носење, проектирана за теретана, патување и урбана средина со флексибилна целодневна удобност.\n\nЕднопарчесна горна страна со CoffeeYarn flyknit за целодневни перформанси, дишливост и удобност.\n\nДоволно елегантна за секојдневно, незгодно и канцелариско носење, но исто така функционална за спорт. Секој пар вклучува еластични врвки кои лесно можете да ги замените.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nCoffeYarn: издржлива, дишлива, антимикробна и брзо сушечка перформанс мрежа, направена од рециклирани кафе зрна.\nРамни врвки — сини со зелени точки или розеви со сиви точки.\n\nОбвивка\nБез обвивка за целосна дишливост.\n\nВлошка\nFlexile: 3,0 мм одредена амортизација со одличен осет за подлогата за целосно природно движење.\n\nСредна потпетица\nSoftFlex: целосно флексибилна за максимално движење на стапалото.\n\nГон\nUrbanGrip: 3,0 мм газен профил со широка површина за оптимален грип и издржливост на урбани површини.\n\nВодоотпорност\nНе е водоотпорно, високо дишливо. Користете водоотпорни чорапи за студени/влажни услови.\n\nТежина\n460 грама на пар во големина 42.\n\nВисина на ѓонот\n9,5 мм со влошка; 6,5 мм без влошка.\n\nНега\nРачно перете со млака вода и благ детергент. Сушете на воздух.\nНе препорачуваме ставање на вашите Freet патики во машина за перење или сушара.',
    E'Горен дел: 65% рециклирано (CoffeeYarn)\nВрвки: 70% рециклирано (BottleYarn)\nСредна потпетица: 70% рециклирано (BottleYarn)\nВлошка: 60% рециклирано\nГон: 60% природен каучук\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија\nНадворешно пакување: рециклирана и рециклабилна пластика или картон/хартиен пакет'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Tanga 3 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'sq',
    'Trajner ultra-i-lehtë dhe i ajrosur për palestër, udhëtime dhe veshje urbane me komfort gjatë gjithë ditës.',
    E'Trajner ultra-i-lehtë dhe i ajrosur i ndërtuar për palestër, udhëtime dhe veshje urbane me komfort të plotë gjatë gjithë ditës.\n\nPjesë e sipërme njëcopësh me CoffeeYarn flyknit për performancë, ajrosje dhe komfort gjatë gjithë ditës.\n\nI mençur për veshje të përditshme, casual dhe zyre, por edhe funksional për sport. Çdo çift përfshin lidhëse elastike që mund t\'i ndërroni lehtë.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nCoffeYarn: rrjetë performance e qëndrueshme, e ajrosur, antimikrobike dhe me tharje të shpejtë, nga lëndët e ricikluara të kafesë.\nLidhëse të sheshta — blu me njolla jeshile ose rozë me njolla gri.\n\nVeshja\nPa veshje për ajrosje të plotë.\n\nSole e brendshme\nFlexile: 3.0 mm njëfarë amortizimi me lidhje të shkëlqyer me tokën për lëvizje natyrale.\n\nMidsole\nSoftFlex: plotësisht fleksibël për lëvizje maksimale të këmbës.\n\nSolla e jashtme\nUrbanGrip: 3.0 mm gjurmë me sipërfaqe të gjerë për grip optimal dhe qëndrueshmëri në sipërfaqe urbane.\n\nPapërshkueshmëria nga uji\nNuk është i papërshkueshëm nga uji, shumë i ajrosur. Përdor çorape të papërshkueshme për kushte të ftohta / të lagështa.\n\nPesha\n460 gramë çift në madhësinë 42.\n\nLartësia e solës\n9.5 mm me sole; 6.5 mm pa sole.\n\nKujdesi\nLani me dorë me ujë të vakët dhe detergjent të butë. Thajeni në ajër.\nNuk rekomandojmë vendosjen e Freet-eve tuaj në lavatriçe ose tharëse.',
    E'Pjesa e sipërme: 65% e ricikluar (CoffeeYarn)\nLidhëset: 70% të ricikluara (BottleYarn)\nMidsole: 70% e ricikluar (BottleYarn)\nSolla e brendshme: 60% e ricikluar\nSolla e jashtme: 60% gomë natyrale\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme\nPaketimi i jashtëm: plastikë ose karton/letër i ricikluar dhe i riciklueshëm'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Tanga 3 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-TANGA-3'), 'en',
    'Ultra-light breathable everyday trainer built for gym, travel and urban wear with flexible all-day comfort.',
    E'Ultra-light breathable everyday trainer built for gym, travel and urban wear with flexible all-day comfort.\n\nSingle piece upper with CoffeeYarn flyknit for all day performance, breathability and comfort.\n\nSmart enough for everyday, casual and office wear, but also functional for sport. Each pair includes elastic laces you can easily swap in.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nCoffeYarn: durable, breathable, antimicrobial and quick drying performance, made from recycled coffee grounds.\nFlat lace — blue with green speckle or pink with grey speckle.\n\nLining\nUnlined for complete breathability.\n\nInsole\nFlexile: 3.0 mm some shock absorption with excellent ground connectivity to allow full natural movement.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nUrbanGrip: 3.0 mm wide-area tread for optimum grip and durability on urban surfaces.\n\nWaterproofness\nNot waterproof, highly breathable. Use a waterproof sock for cold / wet conditions.\n\nWeight\n460 grams per pair size 42.\n\nStack Height\n9.5 mm with insole; 6.5 mm without insole.\n\nCare\nHand wash with luke warm water and mild detergent. Air dry.\nWe do not recommend putting your Freets in the washing machine or dryer.',
    E'Upper: 65% recycled (CoffeeYarn)\nLaces: 70% recycled (BottleYarn)\nMidsole: 70% recycled (BottleYarn)\nInsole: 60% recycled\nOutsole: 60% natural rubber\nPackaging: box/wrapping recycled and recyclable card & paper\nOuter packaging: recycled and recyclable plastic or card/paper packet'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Pace MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-PACE'), 'mk',
    'Перформанс спортска патика за повеќе активности со влошка OrthoLite 6мм за амортизација на потврди, мешани површини.',
    E'Перформанс патика за повеќе активности со нашата влошка OrthoLite 6мм за амортизација на потврди, мешани површини и подолги растојанија.\n\nFlyKnit горен дел изработен од рециклирани кафе зрна за неверојатни перформанси за трчање, планинарење, спорт, теретана или секојдневно носење.\n\nЕдна од нашите најодржливи патики.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nCoffeYarn: издржлива, дишлива, антимикробна и брзо сушечка перформанс мрежа, направена од рециклирани кафе зрна.\nКонвенционални врвки — со точки.\n\nОбвивка\nБез обвивка за целосна дишливост.\n\nВлошка\nOrthoLite: 6,0 мм подлабока, премиум влошка со повисоко ниво на амортизација за потврди патеки и урбани површини и подолги растојанија.\n\nСредна потпетица\nSoftFlex: целосно флексибилна за максимално движење на стапалото.\n\nГон\nMultiGrip: 2,0 мм газен профил обезбедува грип и издржливост на урбани површини и квалитетни патеки.\n\nВодоотпорност\nНе е водоотпорно, високо дишливо. Користете водоотпорни чорапи за студени/влажни услови.\n\nТежина\n452 грама на пар во големина 42.\n\nВисина на ѓонот\n11,5 мм со OrthoLite 6,0 мм влошка; 5,5 мм без влошка. Нула надолен пад.\n\nНега\nРачно перете со млака вода и благ детергент. Сушете на воздух.\nНе препорачуваме ставање на вашите Freet патики во машина за перење или сушара.',
    E'Горен дел: 60% рециклирани кафе зрна (CoffeeYarn)\nГон: 60% природен каучук\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија\nНадворешно пакување: рециклирана и рециклабилна пластика или картон/хартиен пакет'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Pace SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-PACE'), 'sq',
    'Këpucë sportive me shumë aktivitete me solën OrthoLite 6mm për amortizim në sipërfaqe të forta dhe të ndryshme.',
    E'Këpucë sportive me shumë aktivitete me solën tonë OrthoLite 6mm duke dhënë amortizim për sipërfaqe më të forta, të ndryshme dhe distanca më të gjata.\n\nPjesa e sipërme FlyKnit e bërë nga lëndë të ricikluara kafeje për performancë të mrekullueshme, qoftë për vrapim, ecje malore, sport, palestër apo veshje të gjithë ditës.\n\nNjëra nga këpucët tona më të qëndrueshme.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nCoffeYarn: rrjetë performance e qëndrueshme, e ajrosur, antimikrobike dhe me tharje të shpejtë, nga lëndët e ricikluara të kafesë.\nLidhëse konvencionale — me pika.\n\nVeshja\nPa veshje për ajrosje të plotë.\n\nSole e brendshme\nOrthoLite: 6.0 mm një sole e brendshme më e thellë, premium me nivel më të lartë amortizimi për shtigje dhe sipërfaqe urbane më të forta dhe distanca më të gjata.\n\nMidsole\nSoftFlex: plotësisht fleksibël për lëvizje maksimale të këmbës.\n\nSolla e jashtme\nMultiGrip: 2.0 mm gjurmë jep grip dhe qëndrueshmëri në sipërfaqe urbane dhe shtigje të cilësisë së mirë.\n\nPapërshkueshmëria nga uji\nNuk është i papërshkueshëm nga uji, shumë i ajrosur. Përdor çorape të papërshkueshme për kushte të ftohta / të lagështa.\n\nPesha\n452 gramë çift në madhësinë 42.\n\nLartësia e solës\n11.5 mm me OrthoLite 6.0 mm sole; 5.5 mm pa sole. Zero drop.\n\nKujdesi\nLani me dorë me ujë të vakët dhe detergjent të butë. Thajeni në ajër.\nNuk rekomandojmë vendosjen e Freet-eve tuaj në lavatriçe ose tharëse.',
    E'Pjesa e sipërme: 60% lëndë të ricikluara kafeje (CoffeeYarn)\nSolla e jashtme: 60% gomë natyrale\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme\nPaketimi i jashtëm: plastikë ose karton/letër i ricikluar dhe i riciklueshëm'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Pace EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-PACE'), 'en',
    'Performance multi activity sports shoe with our 6mm OrthoLite insole giving shock absorption for harder, mixed surfaces and longer distances.',
    E'Performance multi activity sports shoe with our 6mm OrthoLite insole giving shock absorption for harder, mixed surfaces and longer distances.\n\nFlyKnit upper made from recycled coffee grounds for amazing performance whether for running, hiking, sport, gym or casual all day wear.\n\nOne of our most sustainable shoes.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nCoffeYarn: durable, breathable, antimicrobial and quick drying performance, made from recycled coffee grounds.\nConventional lace — flecked.\n\nLining\nUnlined for complete breathability.\n\nInsole\nOrthoLite: 6.0mm a deeper, premium insole with a higher level of shock absorption for harder trails and urban surfaces and longer distances.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nMultiGrip: 2.0 mm tread gives grip and durability on urban surfaces and good quality trails.\n\nWaterproofness\nNot waterproof, highly breathable. Use a waterproof sock for cold / wet conditions.\n\nWeight\n452 grams per pair size 42.\n\nStack Height\n11.5 mm with OrthoLite 6.0 mm insole; 5.5 mm without insole, zero drop.\n\nCare\nHand wash with luke warm water and mild detergent. Air dry.\nWe do not recommend putting your Freets in the washing machine or dryer.',
    E'Upper: 60% recycled coffee grounds (CoffeeYarn)\nOutsole: 60% natural rubber\nPackaging: box/wrapping recycled and recyclable card & paper\nOuter packaging: recycled and recyclable plastic or card/paper packet'
) ON CONFLICT (product_id, lang) DO NOTHING;
