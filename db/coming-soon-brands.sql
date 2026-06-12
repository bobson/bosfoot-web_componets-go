-- Insert coming-soon brands
INSERT INTO brands (name, sku, slug, is_featured, sort_order) VALUES
  ('Groundies', 'groundies', 'groundies', FALSE, 10),
  ('Xero Shoes', 'xero-shoes', 'xero-shoes', FALSE, 11),
  ('Vivobarefoot', 'vivobarefoot', 'vivobarefoot', FALSE, 12)
ON CONFLICT (slug) DO NOTHING;

-- Add translations for Groundies
INSERT INTO brand_translations (brand_id, lang, description) 
VALUES 
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'mk', 'Прави се за боси обувки со минимално минешување.'),
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'sq', 'Specjalizuar në këpucë barefoot me dizajn minimaliste.'),
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'en', 'Specializing in minimalist barefoot shoes with exceptional ground feel.')
ON CONFLICT (brand_id, lang) DO NOTHING;

-- Add translations for Xero Shoes
INSERT INTO brand_translations (brand_id, lang, description)
VALUES
  ((SELECT id FROM brands WHERE slug = 'xero-shoes'), 'mk', 'Лесни и флексибилни боси патики за активни луѓе.'),
  ((SELECT id FROM brands WHERE slug = 'xero-shoes'), 'sq', 'Këpucë barefoot lehte dhe fleksibël për stilin aktiv.'),
  ((SELECT id FROM brands WHERE slug = 'xero-shoes'), 'en', 'Lightweight and flexible barefoot shoes for active lifestyles.')
ON CONFLICT (brand_id, lang) DO NOTHING;

-- Add translations for Vivobarefoot
INSERT INTO brand_translations (brand_id, lang, description)
VALUES
  ((SELECT id FROM brands WHERE slug = 'vivobarefoot'), 'mk', 'Инова вонредни боси патики со природна динамика.'),
  ((SELECT id FROM brands WHERE slug = 'vivobarefoot'), 'sq', 'Këpucë barefoot inovative me dinamikë natyrore.'),
  ((SELECT id FROM brands WHERE slug = 'vivobarefoot'), 'en', 'Innovative barefoot shoes with exceptional natural dynamics.')
ON CONFLICT (brand_id, lang) DO NOTHING;
