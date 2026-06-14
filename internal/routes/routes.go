package routes

import (
	"bosfoot/handlers"
	"bosfoot/internal/cache"
	"bosfoot/internal/middleware"
	"net/http"
)

func Register(
	productHandler *handlers.ProductHandler,
	orderHandler *handlers.OrderHandler,
	pageHandler *handlers.PageHandler,
	pc *cache.Cache,
) {
	// API handlers (JSON, CSR)
	http.HandleFunc("/api/brands", productHandler.GetBrands)
	http.HandleFunc("/api/products", productHandler.GetProducts)
	http.HandleFunc("/api/products/{id}", productHandler.GetProductByID)

	http.HandleFunc("/api/orders", middleware.WrapCSRF(orderHandler.CreateOrder))

	// Page handlers (HTML, SSR)
	for _, loc := range []string{"mk", "sq", "en"} {
		http.HandleFunc("/"+loc, pc.Wrap(pageHandler.Home))
	}
	http.HandleFunc("/{locale}/products", pc.Wrap(pageHandler.ProductListing))
	http.HandleFunc("/{locale}/products/{brand}/{slug}", pc.Wrap(pageHandler.ProductDetail))
	http.HandleFunc("/{locale}/brands", pc.Wrap(pageHandler.Brands))
	http.HandleFunc("/{locale}/size-guide", pc.Wrap(pageHandler.SizeGuide))
	http.HandleFunc("/{locale}/about", pc.Wrap(pageHandler.About))
	http.HandleFunc("/{locale}/checkout", middleware.WithCSRFCookie(pc.Wrap(pageHandler.Checkout)))
	http.HandleFunc("/{locale}/foot-health", pc.Wrap(pageHandler.FootHealth))
	http.HandleFunc("/{locale}/articles", pc.Wrap(pageHandler.ArticlesListing))
	http.HandleFunc("/{locale}/articles/{slug}", pc.Wrap(pageHandler.ArticleDetail))
	http.HandleFunc("/{locale}/privacy", pc.Wrap(pageHandler.Privacy))
	http.HandleFunc("/{locale}/terms", pc.Wrap(pageHandler.Terms))
	http.HandleFunc("/{locale}/returns", pc.Wrap(pageHandler.Returns))
	http.HandleFunc("/{locale}/shipping", pc.Wrap(pageHandler.Shipping))
	http.HandleFunc("/sitemap.xml", pageHandler.Sitemap)

	// Catch-all
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/mk", http.StatusFound)
			return
		}
		http.FileServer(http.Dir("public")).ServeHTTP(w, r)
	})
}
