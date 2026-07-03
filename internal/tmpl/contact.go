package tmpl

import "html/template"

// ContactInfo is the single source of truth for the shop's public contact
// channels. It's exposed to every template via the `contact` template function,
// so the contact page, footer, and nav drawer all render from here — change a
// number or handle once and it updates everywhere.
//
// The href fields are template.URL (not plain string) because html/template
// sanitises any href whose scheme isn't http/https/mailto down to "#ZgotmplZ" —
// which would silently break the tel: and viber: links. These are our own
// trusted constants, so template.URL tells the engine to emit them verbatim.
//
// The email recipient for the order/notification mailer is configured
// separately via ORDER_NOTIFY_TO (see internal/notify); keep them in sync.
type ContactInfo struct {
	Phone         string       // human-readable, e.g. "+389 70 255 906"
	PhoneHref     template.URL // tel: URI
	Email         string       // address; templates build the mailto: link
	WhatsAppHref  template.URL
	ViberHref     template.URL
	Instagram     string // display handle, e.g. "@bosfoot_store"
	InstagramHref template.URL
	Facebook      string // display label
	FacebookHref  template.URL
}

// SiteContact holds the live Bosfoot contact details. This is the one place to
// edit a phone number, email, or social link.
var SiteContact = ContactInfo{
	Phone:         "+389 70 255 906",
	PhoneHref:     "tel:+38970255906",
	Email:         "info@bosfoot.com",
	WhatsAppHref:  "https://wa.me/38970255906",
	ViberHref:     "viber://add?number=38970255906",
	Instagram:     "@bosfoot_store",
	InstagramHref: "https://www.instagram.com/bosfoot_store",
	Facebook:      "@Bosfoot",
	FacebookHref:  "https://www.facebook.com/profile.php?id=61591117667798",
}
