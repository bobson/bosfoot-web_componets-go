-- ============================================================
-- Bosfoot — barefoot shoes e-commerce store
-- PostgreSQL schema
--
-- Conventions:
--   * Prices stored in MKD only, as whole numbers (no decimals).
--     EUR is calculated at runtime (shown with 2 decimals on sq/en locales).
--   * Languages: mk, sq, en. Product name / SKU, brand name / SKU,
--     colors, spec keys/values, activities and size-chart measurements
--     are always EN. Descriptions, articles, size guides and the
--     how-to-measure guide are translated.
-- ============================================================

-- ---------- Enums ----------

CREATE TYPE lang_code AS ENUM ('mk', 'sq', 'en');

CREATE TYPE order_status AS ENUM ('pending', 'shipped', 'delivered');

CREATE TYPE payment_method AS ENUM ('cod', 'bank_transfer');

-- ---------- Lookup tables ----------

CREATE TABLE brands (
    id                 SERIAL PRIMARY KEY,
    name               TEXT NOT NULL,            -- always EN
    sku                TEXT NOT NULL UNIQUE,     -- always EN
    slug               TEXT NOT NULL UNIQUE,
    country_of_origin  CHAR(2),                  -- ISO code, e.g. 'GB'
    year_founded       INTEGER,
    website_url        TEXT,
    sizing_guide_url   TEXT,                     -- brand's official size guide (linked from size chart)
    logo_url           TEXT,                     -- e.g. '/images/freet/images/freet.svg'
    is_featured        BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order         INTEGER NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Brand description is translated (mk/sq/en); name/sku stay EN on the base table.
CREATE TABLE brand_translations (
    id          SERIAL PRIMARY KEY,
    brand_id    INTEGER NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    lang        lang_code NOT NULL,
    description TEXT,
    UNIQUE (brand_id, lang)
);

CREATE TABLE categories (
    id   SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,           -- shoes, insoles, accessories
    name TEXT NOT NULL
);

CREATE TABLE genders (
    id    SERIAL PRIMARY KEY,
    value TEXT NOT NULL UNIQUE           -- male, female, unisex, kids
);

-- ---------- Products ----------

CREATE TABLE products (
    id                 SERIAL PRIMARY KEY,
    sku                TEXT NOT NULL UNIQUE,    -- always EN
    name               TEXT NOT NULL,           -- always EN
    slug               TEXT NOT NULL,           -- bare form (e.g. 'vibe-2'); unique per brand, see index below
    brand_id           INTEGER NOT NULL REFERENCES brands(id),
    category_id        INTEGER NOT NULL REFERENCES categories(id),
    gender_id          INTEGER NOT NULL REFERENCES genders(id),
    price_mkd          INTEGER NOT NULL,        -- whole MKD, no decimals
    original_price_mkd INTEGER,                 -- strikethrough price, whole MKD
    image_url          TEXT,
    is_new             BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured        BOOLEAN NOT NULL DEFAULT FALSE,
    is_on_sale         BOOLEAN NOT NULL DEFAULT FALSE,
    discount_pct       NUMERIC(5,2),
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    is_published       BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order         INTEGER NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Product slugs are bare (e.g. 'vibe-2') and unique within a brand, matching
-- the URL scheme /products/{brand-slug}/{product-slug}.
CREATE UNIQUE INDEX idx_products_brand_slug ON products(brand_id, slug);

-- Indexes for filtering (brand, gender, category, price, flags)
CREATE INDEX idx_products_brand_id    ON products(brand_id);
CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_gender_id   ON products(gender_id);
CREATE INDEX idx_products_price_mkd   ON products(price_mkd);
CREATE INDEX idx_products_is_new      ON products(is_new)      WHERE is_new;
CREATE INDEX idx_products_is_featured ON products(is_featured) WHERE is_featured;
CREATE INDEX idx_products_is_on_sale  ON products(is_on_sale)  WHERE is_on_sale;

CREATE TABLE product_translations (
    id               SERIAL PRIMARY KEY,
    product_id       INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    lang             lang_code NOT NULL,
    description      TEXT,             -- short description shown on product card
    about            TEXT,             -- about the shoe paragraph(s)
    size_and_fit     TEXT,             -- sizing advice
    product_info     TEXT,             -- full technical details (materials, weight, stack height…)
    sustainability   TEXT,             -- materials/packaging sustainability notes
    meta_title       TEXT,
    meta_description TEXT,
    UNIQUE (product_id, lang)
);

CREATE TABLE product_sizes (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    eu_size    NUMERIC(4,1) NOT NULL,   -- EU sizes, e.g. 38, 39, 40.5
    UNIQUE (product_id, eu_size)
);
CREATE INDEX idx_product_sizes_eu_size ON product_sizes(eu_size);

CREATE TABLE product_colors (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    color      TEXT NOT NULL,           -- always EN
    hex        CHAR(7),                 -- e.g. '#1a1a1a'
    UNIQUE (product_id, color)
);
CREATE INDEX idx_product_colors_color ON product_colors(color);

CREATE TABLE product_activities (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    activity   TEXT NOT NULL,           -- always EN
    UNIQUE (product_id, activity)
);
CREATE INDEX idx_product_activities_activity ON product_activities(activity);

CREATE TABLE product_gallery (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url  TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (product_id, image_url)
);
CREATE INDEX idx_product_gallery_product_id ON product_gallery(product_id);

CREATE TABLE product_specs (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,           -- always EN, e.g. Material, Weight
    value      TEXT NOT NULL,           -- always EN, e.g. Mesh, 250g
    UNIQUE (product_id, key)
);

-- Short feature badges shown on the product card/page (WATERPROOF, GRIPPY…)
-- All three languages stored as columns — highlights always come as a complete set.
CREATE TABLE product_highlights (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    mk         TEXT NOT NULL,
    sq         TEXT NOT NULL,
    en         TEXT NOT NULL,
    UNIQUE (product_id, en)
);
CREATE INDEX idx_product_highlights_product_id ON product_highlights(product_id);

-- Stock tracked per size + color combination
CREATE TABLE product_stock (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size_id    INTEGER NOT NULL REFERENCES product_sizes(id) ON DELETE CASCADE,
    color_id   INTEGER NOT NULL REFERENCES product_colors(id) ON DELETE CASCADE,
    qty        INTEGER NOT NULL DEFAULT 0 CHECK (qty >= 0),
    UNIQUE (product_id, size_id, color_id)
);

-- Per-product size chart (sizes differ even within the same brand)
CREATE TABLE size_chart (
    id              SERIAL PRIMARY KEY,
    product_id      INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    eu_size         NUMERIC(4,1) NOT NULL,
    foot_length_mm  NUMERIC(5,1),
    foot_width_mm   NUMERIC(5,1),
    UNIQUE (product_id, eu_size)
);

-- ---------- Guides ----------

CREATE TABLE brand_size_guide (
    id       SERIAL PRIMARY KEY,
    brand_id INTEGER NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    lang     lang_code NOT NULL,
    content  TEXT NOT NULL,
    UNIQUE (brand_id, lang)
);

CREATE TABLE how_to_measure (
    id      SERIAL PRIMARY KEY,
    lang    lang_code NOT NULL UNIQUE,   -- single global guide per language
    content TEXT NOT NULL
);

-- ---------- Users & orders ----------

CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT,
    address       TEXT,
    lang          lang_code NOT NULL DEFAULT 'en',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- user_id is nullable to allow guest checkout (NULL = guest).
-- Contact + shipping details are snapshotted on every order — for guests
-- because there's no account, and for registered users so the order keeps
-- the address actually used even if the account address changes later.
CREATE TABLE orders (
    id             SERIAL PRIMARY KEY,
    user_id        INTEGER REFERENCES users(id),   -- NULL for guest checkout
    -- contact
    email          TEXT NOT NULL,                  -- order confirmation is sent here
    phone          TEXT,
    -- shipping address snapshot
    first_name     TEXT NOT NULL,
    last_name      TEXT NOT NULL,
    address        TEXT NOT NULL,
    city           TEXT NOT NULL,
    postal_code    TEXT,
    country        CHAR(2) NOT NULL,               -- ISO2; shipping zones: MK, AL, XK, RS, BG, GR
    notes          TEXT,                           -- optional delivery instructions
    -- payment & status
    payment_method payment_method NOT NULL,        -- cod | bank_transfer
    status         order_status NOT NULL DEFAULT 'pending',
    total_mkd      INTEGER NOT NULL,               -- whole MKD, no decimals
    locale         lang_code NOT NULL DEFAULT 'mk',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_email   ON orders(email);
CREATE INDEX idx_orders_status  ON orders(status);

CREATE TABLE order_items (
    id             SERIAL PRIMARY KEY,
    order_id       INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id     INTEGER NOT NULL REFERENCES products(id),
    size           TEXT,
    color          TEXT,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_mkd INTEGER NOT NULL     -- whole MKD, no decimals
);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);

-- ---------- Articles (blog / content) ----------

CREATE TABLE articles (
    id           SERIAL PRIMARY KEY,
    slug         TEXT NOT NULL UNIQUE,  -- EN canonical
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured  BOOLEAN NOT NULL DEFAULT FALSE,
    author       TEXT,
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE article_translations (
    id               SERIAL PRIMARY KEY,
    article_id       INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    lang             lang_code NOT NULL,
    title            TEXT NOT NULL,
    lead             TEXT,
    body             TEXT,  -- JSON array of {style,text} blocks (normal/h2/h3)
    slug             TEXT NOT NULL,
    meta_title       TEXT,
    meta_description TEXT,
    UNIQUE (article_id, lang),
    UNIQUE (lang, slug)
);

-- ---------- Reviews ----------

CREATE TABLE reviews (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    rating     SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reviews_product_id ON reviews(product_id);
