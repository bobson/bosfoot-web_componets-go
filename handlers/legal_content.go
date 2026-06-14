package handlers

import (
	"net/http"

	"bosfoot/internal/locale"
)

// Legal / policy pages (privacy, terms, returns, shipping) are simple static
// content pages rendered from the shared "legal" template. The text below is
// usable BOILERPLATE — review it with a lawyer and replace every [bracketed]
// placeholder (legal entity, address, courier, prices, days) before launch.

type legalSection struct {
	Heading string
	Body    []string // paragraphs
}

type legalDoc struct {
	Title    string
	MetaDesc string
	Updated  string
	Intro    string
	Sections []legalSection
}

// LegalData is the template payload for the shared "legal" template.
type LegalData struct {
	PageBase
	Doc legalDoc
}

// Privacy, Terms, Returns and Shipping are thin wrappers around renderLegal.
func (h *PageHandler) Privacy(w http.ResponseWriter, r *http.Request)  { h.renderLegal(w, r, "privacy") }
func (h *PageHandler) Terms(w http.ResponseWriter, r *http.Request)    { h.renderLegal(w, r, "terms") }
func (h *PageHandler) Returns(w http.ResponseWriter, r *http.Request)  { h.renderLegal(w, r, "returns") }
func (h *PageHandler) Shipping(w http.ResponseWriter, r *http.Request) { h.renderLegal(w, r, "shipping") }

func (h *PageHandler) renderLegal(w http.ResponseWriter, r *http.Request, page string) {
	if !locale.IsValid(r.PathValue("locale")) {
		http.Redirect(w, r, "/"+locale.Default+"/"+page, http.StatusFound)
		return
	}
	loc := locale.FromPath(r.PathValue("locale"))
	doc, ok := legalContent[page][loc]
	if !ok {
		doc = legalContent[page][locale.Default]
	}
	h.Renderer.Render(w, "legal", LegalData{
		PageBase: PageBase{
			Locale:          loc,
			CurrentPath:     "/" + page,
			SiteURL:         h.baseURL(r),
			MetaTitle:       doc.Title,
			MetaDescription: doc.MetaDesc,
		},
		Doc: doc,
	})
}

var legalContent = map[string]map[string]legalDoc{
	"privacy": {
		"mk": {
			Title:    "Политика на приватност",
			MetaDesc: "Како Bosfoot ги собира, користи и штити вашите лични податоци.",
			Updated:  "Последно ажурирано: јуни 2026",
			Intro:    "Оваа политика објаснува кои лични податоци ги собираме кога купувате на Bosfoot, зошто ги собираме и кои се вашите права. Bosfoot е управуван од [правно лице, ЕМБС], со седиште на [адреса].",
			Sections: []legalSection{
				{"Кои податоци ги собираме", []string{"За да ја обработиме вашата нарачка собираме: име и презиме, е-пошта, телефон, адреса за достава и содржина на нарачката. Не чуваме податоци за вашата платежна картичка."}},
				{"Како ги користиме податоците", []string{"Вашите податоци ги користиме исклучиво за обработка и испорака на нарачката, известувања поврзани со нарачката и поддршка на купувачи. Не ги продаваме на трети страни."}},
				{"Споделување со трети страни", []string{"Податоците ги споделуваме само со курирската служба за испорака и, при плаќање со картичка, со овластен платежен процесор — единствено за таа намена."}},
				{"Колачиња", []string{"Користиме само неопходни колачиња за функционирање на кошничката и безбедност (на пр. заштита од CSRF). Не користиме рекламни колачиња."}},
				{"Ваши права", []string{"Имате право на пристап, исправка и бришење на вашите лични податоци. За барање контактирајте нè на info@bosfoot.com."}},
				{"Контакт", []string{"За прашања поврзани со приватноста: info@bosfoot.com."}},
			},
		},
		"sq": {
			Title:    "Politika e privatësisë",
			MetaDesc: "Si i mbledh, përdor dhe mbron Bosfoot të dhënat tuaja personale.",
			Updated:  "Përditësuar së fundi: qershor 2026",
			Intro:    "Kjo politikë shpjegon cilat të dhëna personale mbledhim kur blini te Bosfoot, pse i mbledhim dhe cilat janë të drejtat tuaja. Bosfoot operohet nga [subjekti juridik], me seli në [adresa].",
			Sections: []legalSection{
				{"Cilat të dhëna mbledhim", []string{"Për të përpunuar porosinë tuaj mbledhim: emrin dhe mbiemrin, email-in, telefonin, adresën e dërgesës dhe përmbajtjen e porosisë. Nuk ruajmë të dhënat e kartës suaj të pagesës."}},
				{"Si i përdorim të dhënat", []string{"Të dhënat tuaja i përdorim vetëm për përpunimin dhe dërgimin e porosisë, njoftimet e lidhura me porosinë dhe mbështetjen e klientëve. Nuk i shesim te palët e treta."}},
				{"Ndarja me palë të treta", []string{"Të dhënat i ndajmë vetëm me shërbimin korrier për dërgesë dhe, në rast pagese me kartë, me një procesor pagese të autorizuar — vetëm për atë qëllim."}},
				{"Cookies", []string{"Përdorim vetëm cookies të domosdoshme për funksionimin e shportës dhe sigurinë (p.sh. mbrojtjen CSRF). Nuk përdorim cookies reklamuese."}},
				{"Të drejtat tuaja", []string{"Keni të drejtë të aksesoni, korrigjoni dhe fshini të dhënat tuaja personale. Për kërkesë na kontaktoni në info@bosfoot.com."}},
				{"Kontakt", []string{"Për pyetje rreth privatësisë: info@bosfoot.com."}},
			},
		},
		"en": {
			Title:    "Privacy Policy",
			MetaDesc: "How Bosfoot collects, uses and protects your personal data.",
			Updated:  "Last updated: June 2026",
			Intro:    "This policy explains what personal data we collect when you shop at Bosfoot, why we collect it, and your rights. Bosfoot is operated by [legal entity], registered at [address].",
			Sections: []legalSection{
				{"What we collect", []string{"To process your order we collect your name, email, phone, delivery address and order details. We do not store your payment card data."}},
				{"How we use it", []string{"We use your data only to process and deliver your order, send order-related notifications, and provide customer support. We never sell it to third parties."}},
				{"Sharing with third parties", []string{"We share data only with the courier for delivery and, for card payments, with an authorised payment processor — solely for that purpose."}},
				{"Cookies", []string{"We use only essential cookies needed for the cart to work and for security (e.g. CSRF protection). We do not use advertising cookies."}},
				{"Your rights", []string{"You have the right to access, correct and delete your personal data. To make a request, contact us at info@bosfoot.com."}},
				{"Contact", []string{"For privacy questions: info@bosfoot.com."}},
			},
		},
	},
	"terms": {
		"mk": {
			Title:    "Услови на продажба",
			MetaDesc: "Услови на продажба за нарачки преку Bosfoot.",
			Updated:  "Последно ажурирано: јуни 2026",
			Intro:    "Овие услови важат за сите нарачки направени преку bosfoot.com. Со потврдување на нарачката се согласувате со нив.",
			Sections: []legalSection{
				{"Општо", []string{"Онлајн продавницата Bosfoot е управувана од [правно лице, ЕМБС], [адреса]. Контакт: info@bosfoot.com."}},
				{"Производи и цени", []string{"Сите цени се изразени во македонски денари (МКД) и вклучуваат ДДВ. Цените во евра се само информативни. Нарачката се наплаќа по цената во моментот на потврда."}},
				{"Нарачки", []string{"Нарачката е потврдена кога ќе добиете е-пошта за потврда. Го задржуваме правото да одбиеме или откажеме нарачка во случај на грешка во цена или залиха."}},
				{"Плаќање", []string{"Прифаќаме плаќање при преземање (поуздина) и плаќање со банкарски трансфер. Деталите за трансфер се прикажани при наплата."}},
				{"Достава и враќање", []string{"Информации за роковите и трошоците на достава, како и за враќањето на производите, се достапни на соодветните страници."}},
			},
		},
		"sq": {
			Title:    "Kushtet e shitjes",
			MetaDesc: "Kushtet e shitjes për porositë përmes Bosfoot.",
			Updated:  "Përditësuar së fundi: qershor 2026",
			Intro:    "Këto kushte zbatohen për të gjitha porositë e bëra përmes bosfoot.com. Duke konfirmuar porosinë, ju i pranoni ato.",
			Sections: []legalSection{
				{"Të përgjithshme", []string{"Dyqani online Bosfoot operohet nga [subjekti juridik], [adresa]. Kontakt: info@bosfoot.com."}},
				{"Produktet dhe çmimet", []string{"Të gjitha çmimet janë në denarë maqedonas (MKD) dhe përfshijnë TVSH. Çmimet në euro janë vetëm informuese. Porosia faturohet me çmimin në momentin e konfirmimit."}},
				{"Porositë", []string{"Porosia konsiderohet e konfirmuar kur merrni email-in e konfirmimit. Rezervojmë të drejtën të refuzojmë ose anulojmë një porosi në rast gabimi në çmim ose stok."}},
				{"Pagesa", []string{"Pranojmë pagesë në dorëzim dhe pagesë me transfer bankar. Detajet e transferit shfaqen gjatë arkëtimit."}},
				{"Dërgesa dhe kthimi", []string{"Informacionet për afatet dhe kostot e dërgesës, si dhe për kthimin e produkteve, gjenden në faqet përkatëse."}},
			},
		},
		"en": {
			Title:    "Terms of Sale",
			MetaDesc: "Terms of sale for orders placed through Bosfoot.",
			Updated:  "Last updated: June 2026",
			Intro:    "These terms apply to all orders placed through bosfoot.com. By confirming your order you agree to them.",
			Sections: []legalSection{
				{"General", []string{"The Bosfoot online store is operated by [legal entity], [address]. Contact: info@bosfoot.com."}},
				{"Products and prices", []string{"All prices are in Macedonian denars (MKD) and include VAT. Euro prices are informational only. Your order is charged at the price shown at the time of confirmation."}},
				{"Orders", []string{"An order is confirmed when you receive a confirmation email. We reserve the right to refuse or cancel an order in case of a pricing or stock error."}},
				{"Payment", []string{"We accept cash on delivery and bank transfer. Transfer details are shown at checkout."}},
				{"Delivery and returns", []string{"Delivery times and costs, and information on returns, are available on the respective pages."}},
			},
		},
	},
	"returns": {
		"mk": {
			Title:    "Враќање и поврат на средства",
			MetaDesc: "Враќање во рок од 14 дена и поврат на средства за нарачки од Bosfoot.",
			Updated:  "Последно ажурирано: јуни 2026",
			Intro:    "Сакаме да сте задоволни од вашите барефут патики. Ако нешто не одговара, можете да го вратите.",
			Sections: []legalSection{
				{"Право на откажување", []string{"Имате право да ја откажете нарачката во рок од 14 дена од приемот, без да наведете причина, согласно прописите за заштита на потрошувачите."}},
				{"Услови за враќање", []string{"Производите треба да бидат неносени, во оригинална амбалажа и во состојба погодна за повторна продажба."}},
				{"Како да вратите", []string{"Контактирајте нè на info@bosfoot.com со бројот на нарачката и ќе ви ги дадеме упатствата за враќање. [Трошоците за враќање ги сноси купувачот, освен при дефект.]"}},
				{"Поврат на средства", []string{"По приемот и проверката на вратениот производ, средствата се враќаат во рок од [14] дена, на истиот начин на плаќање или на ваша банкарска сметка."}},
			},
		},
		"sq": {
			Title:    "Kthimi dhe rimbursimi",
			MetaDesc: "Kthim brenda 14 ditësh dhe rimbursim për porositë e Bosfoot.",
			Updated:  "Përditësuar së fundi: qershor 2026",
			Intro:    "Duam të jeni të kënaqur me këpucët tuaja barefoot. Nëse diçka nuk përshtatet, mund ta ktheni.",
			Sections: []legalSection{
				{"E drejta e anulimit", []string{"Keni të drejtë të anuloni porosinë brenda 14 ditësh nga marrja, pa dhënë arsye, sipas rregullave për mbrojtjen e konsumatorit."}},
				{"Kushtet e kthimit", []string{"Produktet duhet të jenë të paveshura, në paketimin origjinal dhe në gjendje të rishitshme."}},
				{"Si të ktheni", []string{"Na kontaktoni në info@bosfoot.com me numrin e porosisë dhe do t'ju japim udhëzimet. [Kostot e kthimit i mbulon blerësi, përveç rasteve me defekt.]"}},
				{"Rimbursimi", []string{"Pas marrjes dhe kontrollit të produktit të kthyer, rimbursimi bëhet brenda [14] ditësh, me të njëjtën mënyrë pagese ose në llogarinë tuaj bankare."}},
			},
		},
		"en": {
			Title:    "Returns & Refunds",
			MetaDesc: "14-day returns and refunds for Bosfoot orders.",
			Updated:  "Last updated: June 2026",
			Intro:    "We want you to be happy with your barefoot shoes. If something doesn't fit, you can return it.",
			Sections: []legalSection{
				{"Right to cancel", []string{"You have the right to cancel your order within 14 days of receipt, without giving a reason, in line with consumer protection rules."}},
				{"Return conditions", []string{"Items must be unworn, in their original packaging, and in resaleable condition."}},
				{"How to return", []string{"Contact us at info@bosfoot.com with your order number and we'll send return instructions. [Return shipping is paid by the customer, except for defective items.]"}},
				{"Refunds", []string{"Once we receive and inspect the returned item, we refund within [14] days, using your original payment method or to your bank account."}},
			},
		},
	},
	"shipping": {
		"mk": {
			Title:    "Достава",
			MetaDesc: "Информации за рокови и трошоци на достава на Bosfoot.",
			Updated:  "Последно ажурирано: јуни 2026",
			Intro:    "Доставуваме низ цела [Северна Македонија] преку [курирска служба].",
			Sections: []legalSection{
				{"Подрачја на достава", []string{"Доставуваме на адреси во [Северна Македонија]. За испораки во странство контактирајте нè на info@bosfoot.com."}},
				{"Време на достава", []string{"Нарачките се обработуваат во рок од [1–2] работни дена. Доставата обично трае [2–4] работни дена."}},
				{"Трошоци", []string{"Трошокот за достава е [___ МКД]. [Бесплатна достава за нарачки над ___ МКД.]"}},
				{"Следење", []string{"По испраќањето ќе добиете информации за следење на пратката."}},
			},
		},
		"sq": {
			Title:    "Dërgesa",
			MetaDesc: "Informacione për afatet dhe kostot e dërgesës te Bosfoot.",
			Updated:  "Përditësuar së fundi: qershor 2026",
			Intro:    "Dërgojmë në të gjithë [Maqedoninë e Veriut] përmes [shërbimit korrier].",
			Sections: []legalSection{
				{"Zonat e dërgesës", []string{"Dërgojmë në adresa në [Maqedoninë e Veriut]. Për dërgesa jashtë vendit na kontaktoni në info@bosfoot.com."}},
				{"Afatet e dërgesës", []string{"Porositë përpunohen brenda [1–2] ditësh pune. Dërgesa zakonisht zgjat [2–4] ditë pune."}},
				{"Kostot", []string{"Kostoja e dërgesës është [___ MKD]. [Dërgesë falas për porositë mbi ___ MKD.]"}},
				{"Gjurmimi", []string{"Pas dërgimit do të merrni informacion për gjurmimin e pakos."}},
			},
		},
		"en": {
			Title:    "Shipping",
			MetaDesc: "Bosfoot delivery times and shipping costs.",
			Updated:  "Last updated: June 2026",
			Intro:    "We deliver across [North Macedonia] via [courier service].",
			Sections: []legalSection{
				{"Delivery areas", []string{"We ship to addresses in [North Macedonia]. For international orders, contact us at info@bosfoot.com."}},
				{"Delivery times", []string{"Orders are processed within [1–2] business days. Delivery usually takes [2–4] business days."}},
				{"Costs", []string{"Shipping costs [___ MKD]. [Free shipping on orders over ___ MKD.]"}},
				{"Tracking", []string{"After dispatch you'll receive tracking information for your parcel."}},
			},
		},
	},
}
