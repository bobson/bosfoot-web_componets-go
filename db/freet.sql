-- ============================================================
-- Freet brand + 5 products
-- Run AFTER schema.sql and seed.sql:
--   psql "$DATABASE_URL" -f import/freet.sql
-- Safe to re-run: ON CONFLICT DO NOTHING / DO UPDATE throughout.
-- ============================================================

-- ---------- Brand ----------

INSERT INTO brands (name, sku, slug, country_of_origin, year_founded, website_url, sizing_guide_url, logo_url, is_featured, sort_order)
VALUES (
    'Freet', 'FREET', 'freet',
    'GB', 2010,
    'https://freetbarefoot.com',
    'https://freetbarefoot.com/pages/size-guide',
    '/images/freet/images/freet.svg',
    TRUE, 1
)
ON CONFLICT (sku) DO NOTHING;

-- ---------- Brand description (translated) ----------

INSERT INTO brand_translations (brand_id, lang, description)
VALUES
    (
        (SELECT id FROM brands WHERE sku = 'FREET'), 'mk',
        'Британски бренд за босоноги патики основан во 2010 година во Северен Јоркшир од Ендру и Сара. Дизајниран за надворешна употреба — планинарење, трчање на патеки, тренинг и секојдневен живот — со широк простор за прстите, флексибилни ѓонови и нула надолен пад. Користи рециклирани материјали (BottleYarn, CoffeeYarn) и трае.'
    ),
    (
        (SELECT id FROM brands WHERE sku = 'FREET'), 'sq',
        'Markë britanike e këpucëve barefoot, e themeluar në vitin 2010 në Yorkshire-in e Veriut nga Andrew dhe Sarah. E dizajnuar për përdorim në natyrë — alpinizëm, vrapim në shtigje, trajnim dhe jetë të përditshme — me hapësirë të gjerë për gishtërinjtë, shollë fleksibël dhe zero drop. Përdor materiale të ricikluara (BottleYarn, CoffeeYarn) dhe ndërtohet për të zgjatur.'
    ),
    (
        (SELECT id FROM brands WHERE sku = 'FREET'), 'en',
        'UK barefoot shoe brand founded in 2010 in North Yorkshire by Andrew and Sarah. Designed for outdoor use — hiking, trail running, training, and everyday life — with wide toe boxes, flexible soles, and zero drop. Uses recycled materials (BottleYarn, CoffeeYarn) and is built to last.'
    )
ON CONFLICT (brand_id, lang) DO NOTHING;

-- ---------- Brand size guide ----------

INSERT INTO brand_size_guide (brand_id, lang, content)
VALUES
    (
        (SELECT id FROM brands WHERE sku = 'FREET'),
        'mk',
        'Должините се за стапалото. Freet препорачува комотно одговарање — повеќето клиенти ја носат својата вообичаена големина.'
    ),
    (
        (SELECT id FROM brands WHERE sku = 'FREET'),
        'sq',
        'Gjatësitë janë për këmbën. Freet rekomandon përshtatje komode — shumica e klientëve mbajnë madhësinë e tyre të zakonshme.'
    ),
    (
        (SELECT id FROM brands WHERE sku = 'FREET'),
        'en',
        'Lengths refer to your foot. Freet recommends a comfortable fit — most customers wear their usual size.'
    )
ON CONFLICT (brand_id, lang) DO NOTHING;

-- ---------- Products ----------

INSERT INTO products (sku, name, slug, brand_id, category_id, gender_id, price_mkd, image_url, is_active, is_published, sort_order)
VALUES
    (
        'FREET-CHAMOIS', 'Chamois', 'chamois',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        12810,
        '/images/freet/chamois/images/brown-black/chamois.webp',
        TRUE, TRUE, 1
    ),
    (
        'FREET-KELD-3', 'Keld 3', 'keld-3',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        5490,
        '/images/freet/keld-3/images/olive/keld-3.webp',
        TRUE, TRUE, 2
    ),
    (
        'FREET-RICHMOND-2', 'Richmond 2', 'richmond-2',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        8540,
        '/images/freet/richmond-2/images/brown-black/richmond-2.webp',
        TRUE, TRUE, 3
    ),
    (
        'FREET-VIBE-2', 'Vibe 2', 'vibe-2',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        6100,
        '/images/freet/vibe-2/images/black/vibe-2.webp',
        TRUE, TRUE, 4
    ),
    (
        'FREET-YORK-2', 'York 2', 'york-2',
        (SELECT id FROM brands WHERE sku = 'FREET'),
        (SELECT id FROM categories WHERE slug = 'shoes'),
        (SELECT id FROM genders WHERE value = 'unisex'),
        8230,
        '/images/freet/york-2/images/black/york-2.webp',
        TRUE, TRUE, 5
    )
ON CONFLICT (sku) DO NOTHING;

-- ---------- Size chart (Freet brand chart applies to all products) ----------

INSERT INTO size_chart (product_id, eu_size, insole_length_mm)
SELECT p.id, s.eu_size, s.len FROM products p
CROSS JOIN (VALUES
    (36, 230), (37, 237), (38, 244), (39, 251), (40, 258),
    (41, 265), (42, 272), (43, 279), (44, 286), (45, 293), (46, 300)
) AS s(eu_size, len)
WHERE p.sku IN ('FREET-CHAMOIS','FREET-KELD-3','FREET-RICHMOND-2','FREET-VIBE-2','FREET-YORK-2')
ON CONFLICT (product_id, eu_size) DO NOTHING;

-- ---------- Sizes ----------

INSERT INTO product_sizes (product_id, eu_size)
SELECT p.id, s.eu_size FROM products p
CROSS JOIN (VALUES
    (36),(37),(38),(39),(40),(41),(42),(43),(44),(45),(46)
) AS s(eu_size)
WHERE p.sku IN ('FREET-CHAMOIS','FREET-KELD-3','FREET-RICHMOND-2','FREET-VIBE-2','FREET-YORK-2')
ON CONFLICT (product_id, eu_size) DO NOTHING;

-- ---------- Colors ----------

INSERT INTO product_colors (product_id, color, hex)
VALUES
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'),    'Brown Black', '#4a2c1a'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'),     'Olive',       '#5e6b3f'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Brown Black', '#5a3825'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'),     'White',       '#f0f0eb'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'),     'Black',       '#1a1a1a'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'),     'Black',       '#1a1a1a')
ON CONFLICT (product_id, color) DO NOTHING;

-- ---------- Activities ----------

INSERT INTO product_activities (product_id, activity)
VALUES
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'),    'hiking'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'),     'hiking'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'),     'running'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'),     'training'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'casual'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'office'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'),     'training'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'),     'casual'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'),     'running'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'),     'casual'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'),     'office')
ON CONFLICT (product_id, activity) DO NOTHING;

-- ---------- Gallery ----------

INSERT INTO product_gallery (product_id, image_url, sort_order)
VALUES
    -- Chamois
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-4.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-5.webp', 5),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-6.webp', 6),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-7.webp', 7),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), '/images/freet/chamois/images/brown-black/chamois-8.webp', 8),
    -- Keld 3
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-4.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-5.webp', 5),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-6.webp', 6),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-7.webp', 7),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), '/images/freet/keld-3/images/olive/keld-3-8.webp', 8),
    -- Richmond 2
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), '/images/freet/richmond-2/images/brown-black/richmond-2-1.webp', 1),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), '/images/freet/richmond-2/images/brown-black/richmond-2-2.webp', 2),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), '/images/freet/richmond-2/images/brown-black/richmond-2-3.webp', 3),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), '/images/freet/richmond-2/images/brown-black/richmond-2-4.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), '/images/freet/richmond-2/images/brown-black/richmond-2-5.webp', 5),
    -- Vibe 2
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-2.webp',  1),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-3.webp',  2),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-5.webp',  3),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-6.webp',  4),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-8.webp',  5),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-9.webp',  6),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/black/vibe-2-10.webp', 7),
    -- Vibe 2 — White colourway (filtered client-side by the /white/ folder in the path)
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-1.webp', 8),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-2.webp', 9),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-3.webp', 10),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-4.webp', 11),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-5.webp', 12),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-6.webp', 13),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), '/images/freet/vibe-2/images/white/vibe-2-7.webp', 14),
    -- York 2
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), '/images/freet/york-2/images/black/york-2-2.webp',  1),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), '/images/freet/york-2/images/black/york-2-5.webp',  2),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), '/images/freet/york-2/images/black/york-2-7.webp',  3),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), '/images/freet/york-2/images/black/york-2-10.webp', 4),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), '/images/freet/york-2/images/black/york-2-11.webp', 5)
ON CONFLICT (product_id, image_url) DO NOTHING;

-- ---------- Product specs (EN key-value, extracted from technical details) ----------

INSERT INTO product_specs (product_id, key, value)
VALUES
    -- Chamois
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Upper',        'Premium full grain leather'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Lining',       'Full waterproof breathable bootee lining'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Insole',       'Flexile Nano 3mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Midsole',      'FirmFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Outsole',      'MountainGrip 6.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Weight',       '860g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Stack Height', '12.5mm with insole / 10mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'Drop',         'Zero drop'),
    -- Keld 3
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Upper',        'BottleYarn breathable mesh (recycled plastic)'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Lining',       'Unlined'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Insole',       'ReBound 4mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Outsole',      'TrailGrip 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Weight',       '435g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Stack Height', '10.5mm with insole / 7.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'Drop',         'Zero drop'),
    -- Richmond 2
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Upper',        'Premium full grain leather'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Lining',       'Breathable moisture-moving lining'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Insole',       'Flexile 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Outsole',      'UrbanGrip 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Weight',       '640g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Stack Height', '9.5mm with insole / 6.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'Drop',         'Zero drop'),
    -- Vibe 2
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Upper',        'BottleYarn breathable mesh (recycled plastic)'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Lining',       'Unlined'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Insole',       'ReBound 4mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Outsole',      'UrbanGrip 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Weight',       '585g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Stack Height', '10.5mm with insole / 6.5mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'Drop',         'Zero drop'),
    -- York 2
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Upper',        'Premium full grain leather'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Lining',       'Breathable moisture-moving lining'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Insole',       'Flexile 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Midsole',      'SoftFlex'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Outsole',      'UrbanGrip 3.0mm'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Weight',       '480g per pair (size 42)'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Stack Height', '9.0mm with insole / 6.0mm without'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'Drop',         'Zero drop')
ON CONFLICT (product_id, key) DO NOTHING;

-- ---------- Highlights ----------

INSERT INTO product_highlights (product_id, sort_order, mk, sq, en)
VALUES
    -- Chamois
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 1, 'Водоотпорно',   'I papërshkueshëm nga uji', 'WATERPROOF'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 2, 'Силен грип',    'Grip i fortë',             'GRIPPY'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 3, 'Лесно',         'I lehtë',                  'LIGHTWEIGHT'),
    ((SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 4, 'Издржливо',     'I qëndrueshëm',            'TOUGH'),
    -- Keld 3
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 1, 'Брзо сушење',   'Tharje e shpejtë',         'QUICK-DRY'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 2, 'Дишливо',       'I ajrosur',                'BREATHABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 3, 'Патека',        'Shteg',                    'TRAIL'),
    -- Richmond 2
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 1, 'Урбано',           'Urban',              'URBAN'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 2, 'Опуштена удобност','Komfort i qetë',     'RELAXED COMFORT'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 3, 'Отпорно на вода',  'Rezistent ndaj ujit','WATER RESISTANT'),
    ((SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 4, 'Издржливо',        'I qëndrueshëm',      'DURABLE'),
    -- Vibe 2
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 1, 'Одржливо',  'I qëndrueshëm', 'SUSTAINABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 2, 'Лесно',     'I lehtë',       'LIGHTWEIGHT'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 3, 'Дишливо',   'I ajrosur',     'BREATHABLE'),
    ((SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 4, 'Издржливо', 'I qëndrueshëm', 'DURABLE'),
    -- York 2
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 1, 'Урбано',          'Urban',              'URBAN'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 2, 'Удобно',          'Komod',              'COMFORT'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 3, 'Отпорно на вода', 'Rezistent ndaj ujit','WATER RESISTANT'),
    ((SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 4, 'Издржливо',       'I qëndrueshëm',      'DURABLE')
ON CONFLICT (product_id, en) DO NOTHING;

-- ---------- Translations ----------

-- Chamois MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'mk',
    'Премиум кожен барефоот чизми дизајниран за најтешките планински предизвици, при тоа прекрасно удобни и исклучително лесни.',
    E'Комбинира удобност, леснотија и издржливост.\n\n6мм MountainGrip ножеви обезбедуваат одличен грип на стрмни кални патеки.\n\nПремиум, мазен кожен горен дел и целосна водоотпорна, дишлива обвивка ја заокружуваат највисоката заштита.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nКожа: премиум, целозрнеста кожа.\nКонвенционални врвки — кафеави.\n\nОбвивка\nЦелосна водоотпорна, дишлива обвивка.\n\nВлошка\nFlexile Nano: полесна, потенка верзија на нашата 3мм Flexile влошка, која дава малку повеќе осет за подлогата и дишливост.\n\nСредна потпетица\nFirmFlex: флексибилност со одредена заштита на потешки, карпести терени.\n\nГон\nMountainGrip: 6,0 мм газен профил за потешки, повлажни патеки и теренот без патека.\n\nВодоотпорност\nВодоотпорна обвивка и водоотпорен горен дел даваат добро ниво на водоотпорност.\n\nТежина\n860 грама на пар во големина 42.\n\nВисина на ѓонот\n12,5 мм со влошка; 10 мм без влошка.\n\nНега\nИзбришете со влажна крпа. Сушете на воздух. Третирајте со кондиционер/водоодбивач погоден за премиум кожа.\n\nНе препорачуваме ставање на вашите Freet патики во машина за перење или сушара.',
    E'Горен дел: премиум целозрнеста кожа со Gold сертификат од Leather Working Group\nВрвки: 70% рециклирано (BottleYarn)\nСредна потпетица: 70% рециклирано (BottleYarn)\nВлошка: 60% рециклирано\nГон: 60% природен каучук\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Chamois SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'sq',
    'Çizme barefoot prej lëkure premium, e dizajnuar për sfidat më të vështira malore, e komode dhe jashtëzakonisht e lehtë në të njëjtën kohë.',
    E'Kombinon komoditetin, lehtësinë dhe qëndrueshmërinë.\n\nThumbat MountainGrip 6mm sigurojnë grip të shkëlqyer në shtigje të pjerrëta dhe me baltë.\n\nLëkurë premium, e lëmuar në pjesën e sipërme dhe veshje plotësisht e papërshkueshme nga uji dhe e ajrosur e plotësojnë nivelin më të lartë të mbrojtjes.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nLëkurë: premium, lëkurë e plotë.\nLidhëse konvencionale — kafe.\n\nVeshja\nVeshje e plotë e papërshkueshme nga uji, e ajrosur.\n\nSole e brendshme\nFlexile Nano: një version më i lehtë, më i hollë i solës sonë Flexile prej 3mm.\n\nMidsole\nFirmFlex: fleksibilitet me njëfarë mbrojtjeje në terrene më të vështira, me gurë.\n\nSolla e jashtme\nMountainGrip: 6,0 mm gjurmë për shtigje më të vështira, më të lagështa.\n\nPesha\n860 gramë çift në madhësinë 42.\n\nLartësia e solës\n12,5 mm me sole të brendshme; 10 mm pa sole të brendshme.\n\nKujdesi\nFshijeni me një copë të lagësht. Thajeni në ajër. Trajtojeni me një kondicioner të përshtatshëm për lëkurë premium.',
    E'Pjesa e sipërme: lëkurë premium me certifikim Gold nga Leather Working Group\nLidhëset: 70% të ricikluara (BottleYarn)\nMidsole: 70% e ricikluar (BottleYarn)\nSolla e brendshme: 60% e ricikluar\nSolla e jashtme: 60% gomë natyrale\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Chamois EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-CHAMOIS'), 'en',
    'Premium leather, barefoot boot designed to take on the toughest mountain challenges, whilst being wonderfully comfortable and extremely lightweight.',
    E'Combines comfort, lightness and toughness.\n\n6mm MountainGrip lugs provide superb grip for steep muddy trails.\n\nPremium, smooth leather upper and full waterproof, breathable lining completes the highest level of protection.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nLeather: premium, full grain leather.\nConventional lace — brown.\n\nLining\nFull waterproof, breathable bootee lining.\n\nInsole\nFlexile Nano: a lighter, thinner version of our 3mm Flexile insole, which gives a little more ground feel and breathability.\n\nMidsole\nFirmFlex: flexibility with some protection on more arduous, rocky terrain.\n\nOutsole\nMountainGrip: 6.0 mm tread for tougher, wetter trails and off track.\n\nWeight\n860 grams per pair size 42.\n\nStack Height\n12.5 mm with insole; 10 mm without insole.\n\nCare\nWipe with a damp cloth. Air dry. Treat with a conditioner/waterproofer suitable for premium leather.',
    E'Upper: Premium full grain leather Gold Certificated by the Leather Working Group\nLaces: 70% recycled (BottleYarn)\nMidsole: 70% recycled (BottleYarn)\nInsole: 60% recycled\nOutsole: 60% natural rubber\nPackaging: box/wrapping recycled and recyclable card & paper'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Keld 3 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'mk',
    'Врвна патика за активности во топло време — лесна, минималистичка и сестрана за патеки и патувања.',
    E'Ултра-минимална сестрана патика за патувања и авантури, погодна за патеки и надвор од нив, со подобрен дизајн.\n\nУживајте во свежината како кај сандала, а истовремено држете ги отпадоците од патеката надвор.\n\nЗаштита комбинирана со флексибилност, мала тежина и барефоот удобност.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nBottleYarn: дишлива, издржлива мрежа направена од рециклирани пластични шишиња.\nКонвенционални врвки — светло маслинести со сини точки.\n\nОбвивка\nБез обвивка за целосна дишливост.\n\nВлошка\nReBound 4 мм: енергетски одѕив со осет за подлогата.\n\nСредна потпетица\nSoftFlex: целосно флексибилна.\n\nГон\nTrailGrip: 3,0 мм газен профил.\n\nВодоотпорност\nНе е водоотпорно. Користете водоотпорни чорапи за студени/влажни услови.\n\nТежина\n435 грама на пар во големина 42.\n\nВисина на ѓонот\n10,5 мм со влошка; 7,5 мм без влошка.',
    E'Горен дел: 60% рециклиран (BottleYarn)\nГон: 60% природен каучук\nОбвивка: 65% рециклирано (BottleYarn)\nВрвки: 70% рециклирано (BottleYarn)\nВлошка: 60% рециклирано\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Keld 3 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'sq',
    'Këpucë maksimale për aventurat në mot të ngrohtë — e lehtë, minimaliste dhe e gjithanshme për shtigje dhe udhëtime.',
    E'Këpucë ultra-minimale, e gjithanshme për udhëtime dhe aventura, e përshtatshme për shtegje dhe jashtë tyre.\n\nShijoni freskinë e një sandali ndërkohë që mbani jashtë mbeturinat e shtegut.\n\nMbrojtje e kombinuar me fleksibilitet, peshë të lehtë dhe komfort barefoot.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nBottleYarn: rrjetë e ajrosur, e qëndrueshme, prej shisheve plastike të ricikluara.\nLidhëse konvencionale — ulli i çelët me njolla blu.\n\nVeshja\nPa veshje për ajrosje të plotë.\n\nSole e brendshme\nReBound 4 mm: kthim energjie me ndjesi të tokës.\n\nMidsole\nSoftFlex: plotësisht fleksibël.\n\nSolla e jashtme\nTrailGrip: 3,0 mm gjurmë.\n\nPapërshkueshmëria nga uji\nNuk është i papërshkueshëm nga uji. Përdor çorape të papërshkueshme për kushte të ftohta.\n\nPesha\n435 gramë çift në madhësinë 42.\n\nLartësia e solës\n10,5 mm me sole; 7,5 mm pa sole.',
    E'Pjesa e sipërme: 60% e ricikluar (BottleYarn)\nSolla e jashtme: 60% gomë natyrale\nVeshja: 65% e ricikluar (BottleYarn)\nLidhëset: 70% të ricikluara (BottleYarn)\nSolla e brendshme: 60% e ricikluar\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Keld 3 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-KELD-3'), 'en',
    'The ultimate performance shoe for adventures in warm weather.',
    E'Ultra-minimal versatile travel and adventure shoe suitable for both on and off the trail with improved design.\n\nEnjoy the coolness of a sandal whilst keeping trail debris out.\n\nProtection combined with flexibility, light weight and barefoot comfort.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nBottleYarn: breathable, durable, moisture moving mesh, made from recycled plastic bottles.\nConventional lace — light olive with blue fleck.\n\nLining\nUnlined for complete breathability.\n\nInsole\nReBound 4 mm: energy return with ground connectivity to allow full natural movement.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nTrailGrip: 3.0 mm tread gives good grip and durability on a variety of trail surfaces.\n\nWaterproofness\nNot waterproof, highly breathable. Use a waterproof sock for cold / wet conditions.\n\nWeight\n435 grams per pair size 42.\n\nStack Height\n10.5 mm with insole; 7.5 mm without insole.',
    E'Upper: 60% recycled (BottleYarn)\nOutsole: 60% natural rubber\nLining: 65% recycled (BottleYarn)\nLace: 70% recycled (BottleYarn)\nInsole: 60% recycled\nPackaging: box/wrapping recycled and recyclable card & paper'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Richmond 2 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'mk',
    'Класична desert чизма за поделовни денови во канцеларија или за опуштено дружење со пријатели.',
    E'Сите придобивки на барефоот, секој ден. Сега ажурирана за поголема удобност и издржливост.\n\nНашата премиум кожа е мека, дишлива, отпорна на вода — обезбедувајќи целодневна удобност.\n\nБезвременски убав изглед во чевел погоден за град и природа.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nКожа: премиум, целозрнеста кожа.\nКонвенционални врвки — кафеави.\n\nОбвивка\nДишлива обвивка што ја носи влагата.\n\nВлошка\nFlexile: 3,0 мм со одредена амортизација и одличен осет за подлогата.\n\nСредна потпетица\nSoftFlex: целосно флексибилна.\n\nГон\nUrbanGrip: 3,0 мм газен профил за урбани површини.\n\nВодоотпорност\nОтпорен на вода кога редовно се третира.\n\nТежина\n640 грама на пар во големина 42.\n\nВисина на ѓонот\n9,5 мм со влошка; 6,5 мм без влошка. Нула надолен пад.',
    E'Врвки: 70% BottleYarn\nЗадник/микро обвивка: 70% BottleYarn\nВлошка: 60% BottleYarn\nГон: 60% природен каучук од одржливо шумарство\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија\nDWR: издржлив третман со водоодбивност. Без употреба на PFC'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Richmond 2 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'sq',
    'Çizme klasike desert për ditë më të zyrtarizuara në zyrë ose për relaksim me miqtë.',
    E'Të gjitha përfitimet e barefoot çdo ditë. Tani e përditësuar për më shumë komfort dhe qëndrueshmëri.\n\nLëkura jonë premium është e butë, e ajrosur, rezistente ndaj ujit, duke dhënë komfort gjatë gjithë ditës.\n\nPamje e bukur pa kohë në një këpucë të përshtatshme për qytet dhe fshat.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nLëkurë: premium, lëkurë e plotë.\nLidhëse konvencionale — kafe.\n\nVeshja\nVeshje e ajrosur që largon lagështinë.\n\nSole e brendshme\nFlexile: 3,0 mm me njëfarë amortizimi dhe ndjesi të shkëlqyer të tokës.\n\nMidsole\nSoftFlex: plotësisht fleksibël.\n\nSolla e jashtme\nUrbanGrip: 3,0 mm gjurmë për sipërfaqe urbane.\n\nPapërshkueshmëria nga uji\nRezistente ndaj ujit kur trajtohet rregullisht.\n\nPesha\n640 gramë çift në madhësinë 42.\n\nLartësia e solës\n9,5 mm me sole; 6,5 mm pa sole. Zero drop.',
    E'Lidhëset: 70% BottleYarn\nMbështetje/mikro-veshje: 70% BottleYarn\nSolla e brendshme: 60% BottleYarn\nSolla e jashtme: 60% gomë natyrale e marrë nga pylltari e qëndrueshme\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme\nDWR: trajtim i qëndrueshëm i papërshkueshmërisë nga uji. Pa përdorim PFC-sh'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Richmond 2 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-RICHMOND-2'), 'en',
    'A classic desert boot for smarter days at the office or casual relaxing with friends.',
    E'All the benefits of barefoot everyday. Now updated for more comfort and durability.\n\nOur premium leather is soft, breathable, water resistant, giving all day comfort.\n\nTimeless good looks in a shoe suitable for town and country.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nLeather: premium, full grain leather.\nConventional lace — brown.\n\nLining\nBreathable, moisture moving lining.\n\nInsole\nFlexile: 3.0 mm some shock absorption with excellent ground connectivity.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nUrbanGrip: 3.0 mm wide-area tread for optimum grip on urban surfaces.\n\nWaterproofness\nWater resistant upper when treated regularly.\n\nWeight\n640 grams per pair size 42.\n\nStack Height\n9.5 mm with insole; 6.5 mm without insole, zero drop.',
    E'Laces: 70% BottleYarn\nBacker/micro lining: 70% BottleYarn\nInsole: 60% BottleYarn\nOutsole: 60% natural rubber tapped from sustainable forestry\nPackaging: box/wrapping recycled and recyclable card & paper\nDWR: Durable water repellency treatment. No use of PFCs'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Vibe 2 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'mk',
    'Дизајнирано за заморни активности, но целосно опуштено за секојдневно носење.',
    E'Flyknit горен дел со удобна обвивка за целодневна удобност со или без чорапи.\n\nВлошка ReBound со енергетски одѕив плус UrbanGrip ѓон даваат одличен осет за подлогата со амортизација.\n\nВисоко дишливо за моменти кога темпото се забрзува.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nBottleYarn: дишлива, издржлива мрежа од рециклирани пластични шишиња.\nКонвенционални врвки — црни.\n\nОбвивка\nБез обвивка за целосна дишливост.\n\nВлошка\nReBound 4 мм: енергетски одѕив со осет за подлогата.\n\nСредна потпетица\nSoftFlex: целосно флексибилна.\n\nГон\nUrbanGrip: 3,0 мм газен профил за урбани површини.\n\nВодоотпорност\nНе е водоотпорно. Користете водоотпорни чорапи за студени/влажни услови.\n\nТежина\n585 грама на пар во големина 42.\n\nВисина на ѓонот\n10,5 мм со влошка; 6,5 мм без влошка. Нула надолен пад.',
    E'Горен дел: 30% рециклирано (BottleYarn)\nГон: 60% природен каучук\nОбвивка: 65% рециклирано (BottleYarn)\nВрвки: 70% рециклирано (BottleYarn)\nВлошка: 60% рециклирано\nПакување: кутија/обвивка од рециклирани и рециклабилни картон и хартија'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Vibe 2 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'sq',
    'E dizajnuar për aktivitete kërkuese, por plotësisht e qetë për përdorim të përditshëm.',
    E'Pjesa e sipërme Flyknit me veshje komode për komfort gjatë gjithë ditës me ose pa çorape.\n\nSolla e brendshme ReBound me kthim energjie plus solla e jashtme UrbanGrip japin ndjesi të shkëlqyer të tokës me amortizim.\n\nShumë e ajrosur për kur ritmi shpejtohet.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nBottleYarn: rrjetë e ajrosur, e qëndrueshme, prej shisheve plastike të ricikluara.\nLidhëse konvencionale — të zeza.\n\nVeshja\nPa veshje për ajrosje të plotë.\n\nSole e brendshme\nReBound 4 mm: kthim energjie me ndjesi të tokës.\n\nMidsole\nSoftFlex: plotësisht fleksibël.\n\nSolla e jashtme\nUrbanGrip: 3,0 mm gjurmë për sipërfaqe urbane.\n\nPapërshkueshmëria nga uji\nNuk është i papërshkueshëm nga uji. Përdor çorape të papërshkueshme.\n\nPesha\n585 gramë çift në madhësinë 42.\n\nLartësia e solës\n10,5 mm me sole; 6,5 mm pa sole. Zero drop.',
    E'Pjesa e sipërme: 30% e ricikluar (BottleYarn)\nSolla e jashtme: 60% gomë natyrale\nVeshja: 65% e ricikluar (BottleYarn)\nLidhëset: 70% të ricikluara (BottleYarn)\nSolla e brendshme: 60% e ricikluar\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- Vibe 2 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-VIBE-2'), 'en',
    'Designed for demanding activity but completely at ease with wearing casually everyday.',
    E'Flyknit upper with comfy lining for all day comfort with or without socks.\n\nReBound energy return insole plus UrbanGrip outsole gives excellent ground feel with shock absorption.\n\nHighly breathable for when the tempo speeds up.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nBottleYarn: breathable, durable, moisture moving mesh, made from recycled plastic bottles.\nConventional lace — black.\n\nLining\nUnlined for complete breathability.\n\nInsole\nReBound 4 mm: energy return with ground connectivity to allow full natural movement.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nUrbanGrip: 3.0 mm wide-area tread for optimum grip and durability on urban surfaces.\n\nWaterproofness\nNot waterproof, highly breathable. Use a waterproof sock for cold / wet conditions.\n\nWeight\n585 grams per pair size 42.\n\nStack Height\n10.5 mm with insole; 6.5 mm without insole, zero drop.',
    E'Upper: 30% recycled (BottleYarn)\nOutsole: 60% natural rubber\nLining: 65% recycled (BottleYarn)\nLace: 70% recycled (BottleYarn)\nInsole: 60% recycled\nPackaging: box/wrapping recycled and recyclable card & paper'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- York 2 MK
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'mk',
    'Класична секојдневна кожна патика со подобрена форма и издржливост. Неверојатна удобност и изглед за долги денови во град.',
    E'Нов издржлив и целосно флексибилен ѓон со страничен шев.\n\nВисоко функционална со отпорност на вода и барефоот удобност.\n\nНајдобрата кожа дава неспоредлива издржливост и безвременски стил.',
    'Купувачите велат дека одговара точно по големината — можете да ја носите вашата вообичаена големина.',
    E'Горен дел\nКожа: премиум, целозрнеста кожа.\nКонвенционални врвки — црни.\n\nОбвивка\nДишлива обвивка што ја носи влагата.\n\nВлошка\nFlexile: 3,0 мм со одредена амортизација и одличен осет за подлогата.\n\nСредна потпетица\nSoftFlex: целосно флексибилна.\n\nГон\nUrbanGrip: 3,0 мм газен профил за урбани површини.\n\nВодоотпорност\nОтпорен на вода кога редовно се третира.\n\nТежина\n480 грама на пар во големина 42.\n\nВисина на ѓонот\n9,0 мм со влошка; 6,0 мм без влошка. Нула надолен пад.',
    E'Кожа: Gold сертификат од Leather Working Group\nВрвки: 70% BottleYarn\nЗадник/микро обвивка: 70% BottleYarn\nВлошка: 60% BottleYarn\nГон: 60% природен каучук од одржливо шумарство\nPakување: кутија/обвивка од рециклирани и рециклабилни картон и хартија\nDWR: издржлив третман со водоодбивност. Без употреба на PFC'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- York 2 SQ
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'sq',
    'Këpucë klasike e përditshme prej lëkure me përshtatje dhe qëndrueshmëri të përmirësuar. Komfort dhe pamje e mahnitshme për ditë të gjata në qytet.',
    E'Sollë e re e qëndrueshme dhe plotësisht fleksibël me qepje anësore.\n\nShumë funksionale me rezistencë ndaj ujit dhe komfort barefoot.\n\nLëkura më e mirë jep qëndrueshmëri të pakrahasueshme dhe stil pa kohë.',
    'Klientët thonë se përshtatet sipas madhësisë — mund të mbani madhësinë tuaj të zakonshme.',
    E'Pjesa e sipërme\nLëkurë: premium, lëkurë e plotë.\nLidhëse konvencionale — të zeza.\n\nVeshja\nVeshje e ajrosur që largon lagështinë.\n\nSole e brendshme\nFlexile: 3,0 mm me njëfarë amortizimi dhe ndjesi të shkëlqyer të tokës.\n\nMidsole\nSoftFlex: plotësisht fleksibël.\n\nSolla e jashtme\nUrbanGrip: 3,0 mm gjurmë për sipërfaqe urbane.\n\nPapërshkueshmëria nga uji\nRezistente ndaj ujit kur trajtohet rregullisht.\n\nPesha\n480 gramë çift në madhësinë 42.\n\nLartësia e solës\n9,0 mm me sole; 6,0 mm pa sole. Zero drop.',
    E'Lëkurë: certifikim Gold nga Leather Working Group\nLidhëset: 70% BottleYarn\nMbështetje/mikro-veshje: 70% BottleYarn\nSolla e brendshme: 60% BottleYarn\nSolla e jashtme: 60% gomë natyrale e marrë nga pylltari e qëndrueshme\nPaketimi: kuti/mbështjellje nga karton dhe letër të ricikluara dhe të riciklueshme\nDWR: trajtim i qëndrueshëm i papërshkueshmërisë nga uji. Pa përdorim PFC-sh'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- York 2 EN
INSERT INTO product_translations (product_id, lang, description, about, size_and_fit, product_info, sustainability)
VALUES (
    (SELECT id FROM products WHERE sku = 'FREET-YORK-2'), 'en',
    'Classic everyday leather shoe with improved fit and durability. Amazing comfort and looks for long days in town.',
    E'New durable and fully flexible outsole with side wall stitching.\n\nHighly functional with water resistance and barefoot comfort.\n\nThe best leather gives unrivalled durability and timeless style.',
    'Customers say it fits true to size — you may stay at your usual size.',
    E'Upper\nLeather: premium, full grain leather.\nConventional lace — black.\n\nLining\nBreathable, moisture moving lining.\n\nInsole\nFlexile: 3.0 mm some shock absorption with excellent ground connectivity.\n\nMidsole\nSoftFlex: fully flexible for maximum foot movement.\n\nOutsole\nUrbanGrip: 3.0 mm wide-area tread for optimum grip and durability on urban surfaces.\n\nWaterproofness\nWater resistant upper when treated regularly.\n\nWeight\n480 grams per pair size 42.\n\nStack Height\n9.0 mm with insole; 6.0 mm without insole, zero drop.',
    E'Leather: Leather Working Group Gold Certificated\nLaces: 70% BottleYarn\nBacker/micro lining: 70% BottleYarn\nInsole: 60% BottleYarn\nOutsole: 60% natural rubber tapped from sustainable forestry\nPackaging: box/wrapping recycled and recyclable card & paper\nDWR: Durable water repellency treatment. No use of PFCs'
) ON CONFLICT (product_id, lang) DO NOTHING;

-- ---------- Stock ----------
-- Pattern from JSON: 40→3, 41→5, 42→5, 43→4, 44→3, all others→0
-- Single-color products get all stock on their one color.
-- Vibe 2 has two colors but the source JSON gives one stock table, so all
-- stock is assigned to Black; White starts at 0 until real counts exist.

INSERT INTO product_stock (product_id, size_id, color_id, qty)
SELECT
    p.id,
    ps.id,
    pc.id,
    CASE
        WHEN p.sku = 'FREET-VIBE-2' AND pc.color = 'White' THEN 0
        ELSE CASE ps.eu_size
            WHEN 40 THEN 3
            WHEN 41 THEN 5
            WHEN 42 THEN 5
            WHEN 43 THEN 4
            WHEN 44 THEN 3
            ELSE 0
        END
    END
FROM products p
JOIN product_sizes  ps ON ps.product_id = p.id
JOIN product_colors pc ON pc.product_id = p.id
WHERE p.sku IN (
    'FREET-CHAMOIS',
    'FREET-KELD-3',
    'FREET-RICHMOND-2',
    'FREET-VIBE-2',
    'FREET-YORK-2'
)
ON CONFLICT (product_id, size_id, color_id) DO NOTHING;
