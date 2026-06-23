package handlers

import (
	"net/http"
	"strings"

	"bosfoot/internal/locale"
)

// wabImgBase is the public path prefix for the "what are barefoot shoes" assets.
const wabImgBase = "/images/what_are_barefoot_shoes"

// barefootBlock is one image+text section of a detail article. On desktop the
// image sits on .Side (left|right); on mobile it stacks above the text.
type barefootBlock struct {
	Image   string // public path to the .webp
	Width   int    // intrinsic px width  (set on <img> to avoid layout shift)
	Height  int    // intrinsic px height
	AltKey  string // locale key for the alt text
	Caption string // literal image attribution, e.g. "Copyright: Vivobarefoot" (optional)
	BodyKey string // locale key; multi-paragraph, rendered with nl2p
	Side    string // "left" | "right" — image side on desktop
}

// barefootArticle drives both the overview teaser row and the detail page for
// one barefoot trait. KeyPrefix namespaces every locale key (e.g. "wab.drop"
// → ".title", ".lead", ".metaTitle", ".blurb", ".teaserAlt").
type barefootArticle struct {
	Slug       string // flat URL slug, identical across locales (e.g. "zero-drop")
	KeyPrefix  string
	TeaserImg  string
	TeaserW    int
	TeaserH    int
	TeaserSide string // "left" | "right" — image side of the overview row
	Blocks     []barefootBlock
}

// barefootArticles is the single source for the overview rows and the three
// detail pages. Order = display order on the overview page.
var barefootArticles = []barefootArticle{
	{
		Slug: "wide-toe-box", KeyPrefix: "wab.toe",
		TeaserImg: wabImgBase + "/wide_toebox.webp", TeaserW: 750, TeaserH: 750, TeaserSide: "left",
		Blocks: []barefootBlock{
			{Image: wabImgBase + "/wide_toebox/baby_foots.webp", Width: 750, Height: 1000, AltKey: "wab.toe.b1.alt", BodyKey: "wab.toe.b1.body", Side: "left"},
			{Image: wabImgBase + "/wide_toebox/barefoot_vs_convetional_foot.webp", Width: 600, Height: 400, AltKey: "wab.toe.b2.alt", Caption: "Copyright: Vivobarefoot", BodyKey: "wab.toe.b2.body", Side: "right"},
			{Image: wabImgBase + "/wide_toebox/vivobarefoot-primus-asana-navy_1.webp", Width: 725, Height: 680, AltKey: "wab.toe.b3.alt", BodyKey: "wab.toe.b3.body", Side: "left"},
		},
	},
	{
		Slug: "zero-drop", KeyPrefix: "wab.drop",
		TeaserImg: wabImgBase + "/zero_drop/zero_drop.webp", TeaserW: 750, TeaserH: 750, TeaserSide: "right",
		Blocks: []barefootBlock{
			{Image: wabImgBase + "/zero_drop/zero_drop.webp", Width: 750, Height: 750, AltKey: "wab.drop.b1.alt", BodyKey: "wab.drop.b1.body", Side: "left"},
			{Image: wabImgBase + "/zero_drop/convetional_shoe.webp", Width: 750, Height: 500, AltKey: "wab.drop.b2.alt", BodyKey: "wab.drop.b2.body", Side: "right"},
			{Image: wabImgBase + "/zero_drop/barefoot_shoe.webp", Width: 750, Height: 500, AltKey: "wab.drop.b3.alt", BodyKey: "wab.drop.b3.body", Side: "left"},
		},
	},
	{
		Slug: "thin-flexible-sole", KeyPrefix: "wab.sole",
		TeaserImg: wabImgBase + "/thin_and_flexible_sole/zero_drop_vs_elevated_heel.webp", TeaserW: 750, TeaserH: 750, TeaserSide: "left",
		Blocks: []barefootBlock{
			{Image: wabImgBase + "/thin_and_flexible_sole/woman_holding_flexed_barefoot_shoe.webp", Width: 750, Height: 937, AltKey: "wab.sole.b1.alt", Caption: "Copyright: Xero Shoes", BodyKey: "wab.sole.b1.body", Side: "left"},
			{Image: wabImgBase + "/thin_and_flexible_sole/zero_drop_vs_elevated_heel.webp", Width: 750, Height: 750, AltKey: "wab.sole.b2.alt", BodyKey: "wab.sole.b2.body", Side: "right"},
			{Image: wabImgBase + "/thin_and_flexible_sole/barefoot_sohes_on_child.webp", Width: 750, Height: 750, AltKey: "wab.sole.b3.alt", Caption: "Copyright: Monkey Tooth", BodyKey: "wab.sole.b3.body", Side: "left"},
		},
	},
}

// barefootOverviewData is the template payload for the overview page.
type barefootOverviewData struct {
	PageBase
	Articles []barefootArticle
}

// barefootDetailData is the template payload for one detail page.
type barefootDetailData struct {
	PageBase
	Article barefootArticle
}

// WhatAreBarefootShoes handles GET /{locale}/what-are-barefoot-shoes — the
// overview page with three alternating image/text rows. Static (no DB).
func (h *PageHandler) WhatAreBarefootShoes(w http.ResponseWriter, r *http.Request) {
	if !locale.IsValid(r.PathValue("locale")) {
		http.Redirect(w, r, "/"+locale.Default+"/what-are-barefoot-shoes", http.StatusFound)
		return
	}
	loc := locale.FromPath(r.PathValue("locale"))
	h.Renderer.Render(w, "what-are-barefoot-shoes", barefootOverviewData{
		PageBase: PageBase{
			Locale:          loc,
			CurrentPath:     "/what-are-barefoot-shoes",
			SiteURL:         h.baseURL(r),
			MetaTitle:       h.UI.T(loc, "wab.metaTitle"),
			MetaDescription: h.UI.T(loc, "wab.metaDescription"),
		},
		Articles: barefootArticles,
	})
}

// BarefootArticle handles the three flat detail routes (/{locale}/zero-drop,
// /wide-toe-box, /thin-flexible-sole). All three register to this handler; the
// slug is the path segment after the locale. Static (no DB).
func (h *PageHandler) BarefootArticle(w http.ResponseWriter, r *http.Request) {
	locParam := r.PathValue("locale")
	slug := strings.TrimPrefix(r.URL.Path, "/"+locParam+"/")
	if !locale.IsValid(locParam) {
		http.Redirect(w, r, "/"+locale.Default+"/"+slug, http.StatusFound)
		return
	}
	loc := locale.FromPath(locParam)

	var article *barefootArticle
	for i := range barefootArticles {
		if barefootArticles[i].Slug == slug {
			article = &barefootArticles[i]
			break
		}
	}
	if article == nil {
		http.NotFound(w, r)
		return
	}

	h.Renderer.Render(w, "barefoot-article", barefootDetailData{
		PageBase: PageBase{
			Locale:          loc,
			CurrentPath:     "/" + slug,
			SiteURL:         h.baseURL(r),
			MetaTitle:       h.UI.T(loc, article.KeyPrefix+".metaTitle"),
			MetaDescription: h.UI.T(loc, article.KeyPrefix+".metaDescription"),
			OGImage:         article.TeaserImg,
		},
		Article: *article,
	})
}
