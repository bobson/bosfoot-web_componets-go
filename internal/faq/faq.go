// Package faq is the single source of truth for the store's questions & answers.
// One trilingual list feeds three things: the /{locale}/faq page, the clickable
// "quick questions" in the assistant widget (instant canned answers, no API),
// and the grounding context for the Claude-backed free-text answers. Keep the
// wording consistent with the legal/policy pages (handlers/legal_content.go).
package faq

import (
	"encoding/json"
	"html/template"
	"strings"
)

// Item is one Q&A in all three languages. Link (optional) is a locale-relative
// path (e.g. "size-finder") turned into /{locale}/size-finder for the consumer.
type Item struct {
	ID        string
	Q         map[string]string
	A         map[string]string
	Link      string            // optional, e.g. "size-finder"
	LinkLabel map[string]string // optional CTA label per language
}

// Local is one item resolved to a single locale, ready for a template or JSON.
type Local struct {
	ID        string `json:"id"`
	Q         string `json:"q"`
	A         string `json:"a"`
	Link      string `json:"link,omitempty"`      // full path, e.g. /mk/size-finder
	LinkLabel string `json:"linkLabel,omitempty"` // localized CTA label
}

func pick(m map[string]string, locale string) string {
	if v, ok := m[locale]; ok && v != "" {
		return v
	}
	if v, ok := m["mk"]; ok && v != "" {
		return v
	}
	return m["en"]
}

// Localized returns every Q&A resolved to the given locale, in display order.
func Localized(locale string) []Local {
	out := make([]Local, 0, len(Items))
	for _, it := range Items {
		l := Local{ID: it.ID, Q: pick(it.Q, locale), A: pick(it.A, locale)}
		if it.Link != "" {
			l.Link = "/" + locale + "/" + it.Link
			l.LinkLabel = pick(it.LinkLabel, locale)
		}
		out = append(out, l)
	}
	return out
}

// GroundingText renders all Q&A for a locale as a plain-text block for the
// Claude system prompt, so free-text answers stay accurate and on-brand.
func GroundingText(locale string) string {
	var b strings.Builder
	for _, l := range Localized(locale) {
		b.WriteString("Q: ")
		b.WriteString(l.Q)
		b.WriteString("\nA: ")
		b.WriteString(l.A)
		b.WriteString("\n\n")
	}
	return b.String()
}

// SchemaJSON returns a schema.org FAQPage JSON-LD document for SEO rich results.
func SchemaJSON(locale string) template.JS {
	type answer struct {
		Type string `json:"@type"`
		Text string `json:"text"`
	}
	type entity struct {
		Type           string `json:"@type"`
		Name           string `json:"name"`
		AcceptedAnswer answer `json:"acceptedAnswer"`
	}
	items := Localized(locale)
	ents := make([]entity, 0, len(items))
	for _, l := range items {
		ents = append(ents, entity{
			Type:           "Question",
			Name:           l.Q,
			AcceptedAnswer: answer{Type: "Answer", Text: l.A},
		})
	}
	doc := map[string]any{
		"@context":   "https://schema.org",
		"@type":      "FAQPage",
		"mainEntity": ents,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

// Items — edit copy here; it updates the FAQ page, the widget, and the bot.
var Items = []Item{
	{
		ID: "size",
		Q: map[string]string{
			"mk": "Кој број ми одговара?",
			"sq": "Cili numër më përshtatet?",
			"en": "Which size fits me?",
		},
		A: map[string]string{
			"mk": "Измери ја должината на стапалото во милиметри со линијар, потоа додади најмалку 7 мм (0,7 см) простор за прстите и спореди ја таа должина со табелата за внатрешна должина (мм) на секој производ. Барефут патиките се мерат според вистинската внатрешна должина, не според општ ЕУ број. Ако стапалата ти се различни, избери го бројот за подолгото.",
			"sq": "Mate gjatësinë e këmbës në milimetra me vizore, pastaj shto të paktën 7 mm (0,7 cm) hapësirë për gishtat dhe krahasoje me tabelën e gjatësisë së brendshme (mm) në secilën faqe produkti. Këpucët barefoot maten sipas gjatësisë reale të brendshme, jo sipas një numri të përgjithshëm EU. Nëse këmbët të ndryshojnë, zgjidh numrin për këmbën më të gjatë.",
			"en": "Measure your foot length in millimetres with a ruler, then add at least 7 mm (0.7 cm) of toe room and match that fit length to the internal-length (mm) chart on each product page. Barefoot shoes are sized by real internal length, not a generic EU number. If your feet differ, choose the size for the longer foot.",
		},
	},
	{
		ID: "fit",
		Q: map[string]string{
			"mk": "Што ако не ми чинат чевлите?",
			"sq": "Po nëse këpucët nuk më rrinë?",
			"en": "What if the shoes don't fit?",
		},
		A: map[string]string{
			"mk": "Имаш 14 дена да ги вратиш или замениш. За друг број, новиот пар ти го испраќаме бесплатно — го покриваш само трошокот за враќање на првиот пар. Патиките мора да се неносени (пробани само дома), со етикети и во неоштетена кутија. Пиши ни на info@bosfoot.com со бројот на нарачката.",
			"sq": "Ke 14 ditë për t'i kthyer ose ndërruar. Për një numër tjetër, palën e re ta dërgojmë falas — mbulon vetëm koston e kthimit të palës së parë. Këpucët duhet të jenë të pambajtura (të provuara vetëm brenda), me etiketa dhe në kutinë e padëmtuar. Na shkruaj në info@bosfoot.com me numrin e porosisë.",
			"en": "You have 14 days to return or exchange. For a different size we ship the new pair to you free — you only cover sending the original pair back. Shoes must be unworn (tried indoors only), with tags and the undamaged box. Just email info@bosfoot.com with your order number.",
		},
	},
	{
		ID: "delivery",
		Q: map[string]string{
			"mk": "Колку е достава и колку трае?",
			"sq": "Sa kushton dërgesa dhe sa zgjat?",
			"en": "How much is delivery and how long does it take?",
		},
		A: map[string]string{
			"mk": "Доставата е 250 ден, а бесплатна за нарачки над 3.500 ден. Доставуваме низ цела Македонија преку Карго Експрес и обично пристигнува во рок од 2–5 работни дена.",
			"sq": "Dërgesa është 250 den, dhe falas për porositë mbi 3.500 den. Dërgojmë në të gjithë Maqedoninë me Kargo Ekspres dhe zakonisht mbërrin brenda 2–5 ditëve pune.",
			"en": "Delivery is 250 den, and free for orders over 3.500 den. We ship across North Macedonia with Kargo Ekspres and it usually arrives within 2–5 working days.",
		},
	},
	{
		ID: "payment",
		Q: map[string]string{
			"mk": "Како плаќам?",
			"sq": "Si paguaj?",
			"en": "How do I pay?",
		},
		A: map[string]string{
			"mk": "Плаќаш при испорака (готовина на курирот) или со банкарски трансфер — ништо однапред. За трансфер ти ги испраќаме деталите по нарачката и испорачуваме по потврда на уплатата.",
			"sq": "Paguan me dorëzim (kesh te korrieri) ose me transfer bankar — asgjë paraprakisht. Për transfer të dërgojmë detajet pas porosisë dhe dërgojmë pas konfirmimit të pagesës.",
			"en": "You pay on delivery (cash to the courier) or by bank transfer — nothing upfront. For bank transfer we send the details after the order and ship once payment is confirmed.",
		},
	},
	{
		ID: "authentic",
		Q: map[string]string{
			"mk": "Дали чевлите се оригинални?",
			"sq": "A janë këpucët origjinale?",
			"en": "Are the shoes original?",
		},
		A: map[string]string{
			"mk": "Да. Ние сме барефут продавница со внимателно избрани модели — секој пар е вистински, оригинален производ набавен директно од брендот. Не произведуваме свои патики; избираме добри и стоиме зад нив.",
			"sq": "Po. Ne jemi një dyqan barefoot i kuruar — çdo palë është produkt i vërtetë, origjinal, i furnizuar drejtpërdrejt nga marka. Nuk prodhojmë këpucët tona; zgjedhim të mira dhe qëndrojmë pas tyre.",
			"en": "Yes. We're a curated barefoot store — every pair is a genuine, original product sourced directly from the brand. We don't make our own shoes; we choose good ones and stand behind them.",
		},
	},
	{
		ID:   "barefoot",
		Link: "foot-health",
		LinkLabel: map[string]string{
			"mk": "Прочитај го водичот", "sq": "Lexo udhëzuesin", "en": "Read the guide",
		},
		Q: map[string]string{
			"mk": "Што се барефут чевли — дали се за мене?",
			"sq": "Çfarë janë këpucët barefoot — a janë për mua?",
			"en": "What are barefoot shoes — are they for me?",
		},
		A: map[string]string{
			"mk": "Барефут чевлите имаат широк простор за прстите, рамен ѓон (нула дроп) и тенок, флексибилен ѓон, за да може стапалото да се движи и да го чувствува тлото природно. Одговараат на повеќето луѓе; ако си нов, навикнувај се постепено.",
			"sq": "Këpucët barefoot kanë hapësirë të gjerë për gishtat, shollë të sheshtë (zero drop) dhe shollë të hollë e fleksibël, që këmba të lëvizë dhe ta ndiejë tokën natyrshëm. U përshtaten shumicës; nëse je i ri me to, mësohu gradualisht.",
			"en": "Barefoot shoes have a wide toe box, a flat (zero-drop) sole and a thin, flexible sole, so your foot can move and feel the ground naturally. They suit most people; if you're new to them, ease in gradually.",
		},
	},
	{
		ID: "exchange",
		Q: map[string]string{
			"mk": "Може ли да заменам за друг број?",
			"sq": "A mund ta ndërroj për një numër tjetër?",
			"en": "Can I exchange for another size?",
		},
		A: map[string]string{
			"mk": "Да — во рок од 14 дена. Пиши ни на info@bosfoot.com со бројот на нарачката и бројот што ти треба; замената ти ја испраќаме бесплатно (го покриваш само враќањето на првиот пар). Патиките мора да се неносени, со етикети и во оригиналната кутија.",
			"sq": "Po — brenda 14 ditësh. Na shkruaj në info@bosfoot.com me numrin e porosisë dhe numrin që të duhet; ndërrimin ta dërgojmë falas (mbulon vetëm kthimin e palës së parë). Këpucët duhet të jenë të pambajtura, me etiketa dhe në kutinë origjinale.",
			"en": "Yes — within 14 days. Email info@bosfoot.com with your order number and the size you need; we ship the replacement free (you cover only the return of the first pair). Shoes must be unworn, with tags and the original box.",
		},
	},
	{
		ID: "defective",
		Q: map[string]string{
			"mk": "Што ако производот е оштетен или погрешен?",
			"sq": "Po nëse produkti është i dëmtuar ose i gabuar?",
			"en": "What if my item is damaged or wrong?",
		},
		A: map[string]string{
			"mk": "Ако парот пристигне дефектен, или сме испратиле погрешен артикл, целосно е на нас — ги покриваме сите трошоци за враќање и замена, и ти испраќаме нов пар или ти ги враќаме парите во целост. Само контактирај нè на info@bosfoot.com.",
			"sq": "Nëse pala mbërrin me defekt, ose kemi dërguar artikull të gabuar, është plotësisht mbi ne — mbulojmë të gjitha shpenzimet e kthimit dhe ndërrimit, dhe të dërgojmë një palë të re ose t'i kthejmë paratë e plota. Thjesht na kontakto në info@bosfoot.com.",
			"en": "If a pair arrives defective, or we sent the wrong item, it's entirely on us — we cover all return and replacement shipping and send a new pair or refund you in full. Just contact info@bosfoot.com.",
		},
	},
	{
		ID: "store",
		Q: map[string]string{
			"mk": "Каде сте — има ли продавница?",
			"sq": "Ku jeni — a ka dyqan?",
			"en": "Where are you — is there a store?",
		},
		A: map[string]string{
			"mk": "Ние сме онлајн продавница со седиште во Охрид, Северна Македонија — засега немаме физички салон. Прашања? Пиши на info@bosfoot.com или јави се на Вибер.",
			"sq": "Ne jemi një dyqan online me seli në Ohër, Maqedonia e Veriut — për momentin nuk kemi sallon fizik. Pyetje? Shkruaj në info@bosfoot.com ose na kontakto në Viber.",
			"en": "We're an online shop based in Ohrid, North Macedonia — there's no physical showroom for now. Questions? Write to info@bosfoot.com or reach us on Viber.",
		},
	},
}
