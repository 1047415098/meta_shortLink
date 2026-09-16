<template>
  <div class="landing-page" id="top">
    <header class="site-header">
      <div class="wrap header-inner">
        <a class="brand" href="#top" aria-label="Back to the top">
          <img src="/landing-assets/images/mark.svg" alt="" width="42" height="42" />
          <span class="brand-copy">
            <strong>{{ link.landing_brand }}</strong>
            <small>Research materials</small>
          </span>
        </a>

        <!-- All visible consultation entries share the signed contact form below. -->
        <button v-if="ticket" class="header-contact" type="submit" form="contact-form" aria-label="Chat on WhatsApp">
          <WhatsAppIcon />
          <span>WhatsApp</span>
        </button>
        <a v-else class="header-contact" :href="link.target_url" rel="noreferrer" aria-label="Chat on WhatsApp">
          <WhatsAppIcon />
          <span>WhatsApp</span>
        </a>
      </div>
    </header>

    <section class="hero">
      <div class="wrap hero-content">
        <div class="eyebrow">The research collection</div>
        <h1>{{ link.landing_title }}</h1>
        <p class="description">{{ link.landing_description }}</p>

        <div class="reply-status">
          <span aria-hidden="true"></span>
          Typically replies in under 5 minutes
        </div>

        <template v-if="ticket">
          <form id="contact-form" ref="contactForm" method="post" :action="`/${encodeURIComponent(link.code)}/contact`">
            <input id="contact-trigger" ref="contactTrigger" type="hidden" name="trigger" value="manual" />
            <input type="hidden" name="ticket" :value="ticket" />
            <button class="primary-contact" type="submit">
              <WhatsAppIcon />
              <span>
                <strong>Chat on WhatsApp</strong>
                <small>Get Instant Quote &amp; COA</small>
              </span>
            </button>
          </form>
        </template>
        <a v-else class="primary-contact" :href="link.target_url" rel="noreferrer">
          <WhatsAppIcon />
          <span>
            <strong>Chat on WhatsApp</strong>
            <small>Get Instant Quote &amp; COA</small>
          </span>
        </a>

        <small v-if="landingData.meta_measurement === true" class="measurement-notice">
          By clicking, you will be redirected to WhatsApp. Meta ads tracking applies.
        </small>
        <small v-if="ticket && remaining !== null" class="countdown-notice">
          Continuing to WhatsApp in <strong>{{ remaining }}</strong> seconds.
        </small>

        <a class="explore-link" href="#products">
          Explore products
          <span aria-hidden="true">↓</span>
        </a>
      </div>
    </section>

    <main>
      <section class="wrap products-section" id="products">
        <div class="section-head">
          <div>
            <h2>Selected research products</h2>
            <p>A closer look at the PEPLYRA product catalog.</p>
          </div>
          <a class="catalog-link" href="https://peplyra.com/products" target="_blank" rel="noopener noreferrer">
            View the full catalog ↗
          </a>
        </div>

        <div class="products-grid">
          <article v-for="product in products" :key="product.name" class="product-card">
            <div class="product-image">
              <img :src="product.image" :alt="`${product.name} research product`" width="640" height="640" loading="lazy" decoding="async" />
            </div>
            <div class="product-copy">
              <span class="badge">RESEARCH USE ONLY</span>
              <h3>{{ product.name }}</h3>
              <p>{{ product.description }}</p>

              <!-- Product enquiries use the same manual consultation counter as the hero CTA. -->
              <button v-if="ticket" class="product-contact" type="submit" form="contact-form" :aria-label="`Enquire about ${product.name} on WhatsApp`">
                <WhatsAppIcon />
                <span>Enquire now</span>
              </button>
              <a v-else class="product-contact" :href="link.target_url" rel="noreferrer" :aria-label="`Enquire about ${product.name} on WhatsApp`">
                <WhatsAppIcon />
                <span>Enquire now</span>
              </a>
            </div>
          </article>
        </div>
      </section>

      <section v-if="link.landing_details" class="wrap research-details">
        <div class="eyebrow">Information before enquiry</div>
        <h2>Details for your research.</h2>
        <p>{{ link.landing_details }}</p>
      </section>
    </main>

    <!-- The floating mobile action keeps consultation within thumb reach. -->
    <aside class="floating-contact" aria-label="WhatsApp consultation">
      <div class="floating-copy">
        <span class="floating-icon"><WhatsAppIcon /></span>
        <span>
          <strong>Chat with a Specialist</strong>
          <small>Online now</small>
        </span>
      </div>
      <button v-if="ticket" class="floating-button" type="submit" form="contact-form">Chat Now</button>
      <a v-else class="floating-button" :href="link.target_url" rel="noreferrer">Chat Now</a>
    </aside>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from "vue";
import WhatsAppIcon from "./components/WhatsAppIcon.vue";
import { landingData } from "../bootstrap.js";
import { createContactCountdown, submitLandingContact } from "../lib/contact.js";
import { buildMetaAttributionHeaders } from "../lib/attribution.js";
import { reportLandingView } from "../lib/view.js";

const link = landingData.link;
const ticket = landingData.ticket || "";
const contactForm = ref(null);
const contactTrigger = ref(null);
const remaining = ref(null);

// Product copy stays local to this landing page because it has no admin-side behavior.
const products = [
  {
    name: "Tirzepatide 10mg",
    description: "Product specifications and batch information available on enquiry.",
    image: "/landing-assets/images/tirzepatide.webp",
  },
  {
    name: "Semaglutide 5mg",
    description: "Research material details and documentation available on enquiry.",
    image: "/landing-assets/images/semaglutide.webp",
  },
  {
    name: "Retatrutide 10mg",
    description: "Ask about current specifications, batches and availability.",
    image: "/landing-assets/images/retatrutide.webp",
  },
  {
    name: "AOD9604",
    description: "Request product information and available batch documents.",
    image: "/landing-assets/images/aod9604.webp",
  },
  {
    name: "Cagrilintide",
    description: "Discuss research specifications and current availability.",
    image: "/landing-assets/images/cagrilintide.webp",
  },
  {
    name: "Tesamorelin",
    description: "Ask about formats, documentation and delivery options.",
    image: "/landing-assets/images/tesamorelin.webp",
  },
];

// The initial URL remains the source of Meta attribution headers for every landing request.
const attributionHeaders = buildMetaAttributionHeaders();
let cleanup;

// Fetch records the consultation before navigation; the signed native form is the resilient fallback.
async function handleContact(trigger) {
  if (contactTrigger.value) contactTrigger.value.value = trigger;
  try {
    await submitLandingContact({ code: link.code, ticket, trigger, attributionHeaders });
  } catch {
    contactForm.value?.submit();
  }
}

onMounted(() => {
  // The confirmed page view carries the same Meta attribution mirror as consultation requests.
  reportLandingView({ code: link.code, ticket, attributionHeaders });
  document.title = `${link.landing_title} | ${link.landing_brand}`;
  for (const [selector, content] of [
    ['meta[name="description"]', link.landing_description],
    ['meta[property="og:title"]', link.landing_title],
    ['meta[property="og:description"]', link.landing_description],
  ]) {
    document.querySelector(selector)?.setAttribute("content", content || "");
  }
  if (!ticket || !contactForm.value) return;
  cleanup = createContactCountdown({
    form: contactForm.value,
    trigger: contactTrigger.value,
    delay: link.landing_delay,
    restored: performance.getEntriesByType("navigation")[0]?.type === "back_forward",
    onRemaining: (value) => {
      remaining.value = value;
    },
    onSubmit: (trigger) => void handleContact(trigger),
  });
});

onBeforeUnmount(() => cleanup?.());
</script>

<style scoped>
.landing-page {
  min-height: 100vh;
  padding-bottom: 104px;
  color: #102d4f;
  background: #ffffff;
}
.wrap {
  width: min(100%, 1160px);
  margin: 0 auto;
  padding-right: 28px;
  padding-left: 28px;
}
.site-header {
  position: relative;
  z-index: 3;
  background: rgba(255, 255, 255, 0.96);
  border-bottom: 1px solid #e8edf1;
}
.header-inner {
  display: flex;
  min-height: 88px;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 11px;
  color: #071b34;
  text-decoration: none;
}
.brand img {
  width: 42px;
  height: 42px;
  flex: 0 0 auto;
}
.brand-copy {
  display: block;
  min-width: 0;
}
.brand strong {
  display: block;
  overflow: hidden;
  font-size: 17px;
  font-weight: 800;
  line-height: 1.2;
  letter-spacing: 0.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.brand small {
  display: block;
  margin-top: 6px;
  color: #648190;
  font-size: 9px;
  letter-spacing: 3px;
  text-transform: uppercase;
}
.header-contact,
.product-contact,
.floating-button {
  border: 0;
  color: #ffffff;
  background: #20bf55;
  text-decoration: none;
  cursor: pointer;
}
.header-contact {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 8px 13px;
  border-radius: 5px;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}
.header-contact :deep(svg) {
  width: 20px;
  height: 20px;
}
.hero {
  position: relative;
  overflow: hidden;
  background:
    linear-gradient(90deg, rgba(238, 247, 252, 0.98) 0%, rgba(238, 247, 252, 0.88) 44%, rgba(238, 247, 252, 0.48) 100%),
    url("/landing-assets/images/hero-79e77a51693b3249.webp") center / cover no-repeat;
}
.hero-content {
  position: relative;
  z-index: 1;
  padding-top: 55px;
  padding-bottom: 48px;
}
.eyebrow {
  display: flex;
  align-items: center;
  gap: 11px;
  color: #317a83;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 2px;
  text-transform: uppercase;
}
.eyebrow::before {
  width: 25px;
  height: 2px;
  background: currentColor;
  content: "";
}
h1 {
  max-width: 680px;
  margin: 20px 0 17px;
  color: #075895;
  font-size: clamp(42px, 5.3vw, 64px);
  font-weight: 850;
  line-height: 1.04;
  letter-spacing: -2.2px;
  overflow-wrap: anywhere;
}
.description {
  max-width: 620px;
  margin: 0;
  color: #14263a;
  font-size: 16px;
  line-height: 1.75;
  white-space: pre-line;
  overflow-wrap: anywhere;
}
.reply-status {
  display: flex;
  max-width: 620px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin: 20px 0 8px;
  color: #25313d;
  font-size: 13px;
}
.reply-status span {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #20bf55;
  box-shadow: 0 0 0 3px rgba(32, 191, 85, 0.1);
}
#contact-form {
  max-width: 620px;
}
.primary-contact {
  display: flex;
  width: 100%;
  max-width: 620px;
  min-height: 68px;
  align-items: center;
  justify-content: center;
  gap: 17px;
  padding: 9px 24px;
  border: 0;
  border-radius: 6px;
  color: #ffffff;
  background: #19bf50;
  box-shadow: 0 8px 20px rgba(11, 127, 58, 0.2);
  text-align: center;
  text-decoration: none;
  cursor: pointer;
}
.primary-contact :deep(svg) {
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
}
.primary-contact span {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}
.primary-contact strong {
  font-size: 20px;
  line-height: 1.2;
}
.primary-contact small {
  margin-top: 1px;
  font-size: 13px;
  line-height: 1.2;
}
.measurement-notice,
.countdown-notice {
  display: block;
  max-width: 620px;
  margin-top: 8px;
  color: #65717c;
  font-size: 11px;
  line-height: 1.45;
}
.countdown-notice strong {
  color: #075895;
}
.explore-link {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-top: 20px;
  color: #152b43;
  font-size: 13px;
  text-decoration: none;
}
.products-section {
  /* Defer below-fold paint and layout while preserving scroll geometry. */
  content-visibility: auto;
  contain-intrinsic-size: auto 900px;
  padding-top: 48px;
  padding-bottom: 54px;
}
.section-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}
h2 {
  margin: 0;
  color: #101820;
  font-size: 31px;
  font-weight: 850;
  line-height: 1.2;
  letter-spacing: -0.9px;
}
.section-head p {
  margin: 8px 0 0;
  color: #415166;
  font-size: 14px;
}
.catalog-link {
  flex: 0 0 auto;
  padding-bottom: 4px;
  border-bottom: 1px solid #9cb5bd;
  color: #347881;
  font-size: 13px;
  text-decoration: none;
}
.products-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 22px;
}
.product-card {
  overflow: hidden;
  border: 1px solid #dce3e8;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(17, 40, 66, 0.05);
}
.product-image {
  position: relative;
  aspect-ratio: 1.35;
  overflow: hidden;
  border-bottom: 1px solid #e1e6ea;
  background: #f5f6f7;
}
.product-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  transition: transform 180ms ease;
}
.product-card:hover .product-image img {
  transform: scale(1.025);
}
.badge {
  display: inline-flex;
  align-self: flex-start;
  margin-bottom: 10px;
  padding: 5px 8px;
  border-radius: 3px;
  color: #ffffff;
  background: #377f83;
  font-size: 8px;
  font-weight: 800;
  letter-spacing: 0.5px;
}
.product-copy {
  display: flex;
  min-height: 178px;
  flex-direction: column;
  padding: 16px;
}
.product-copy h3 {
  margin: 0;
  color: #101820;
  font-size: 18px;
  font-weight: 850;
  line-height: 1.2;
}
.product-copy p {
  flex: 1;
  margin: 8px 0 15px;
  color: #3f4a57;
  font-size: 12px;
  line-height: 1.45;
}
.product-contact {
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 5px;
  font-size: 14px;
  font-weight: 800;
}
.product-contact :deep(svg) {
  width: 21px;
  height: 21px;
}
.research-details {
  margin-bottom: 38px;
  padding-top: 34px;
  padding-bottom: 34px;
  border-top: 1px solid #dce4e9;
}
.research-details h2 {
  margin-top: 14px;
  color: #102d4f;
}
.research-details p {
  max-width: 800px;
  margin: 18px 0 0;
  color: #536274;
  font-size: 14px;
  line-height: 1.8;
  white-space: pre-line;
}
.floating-contact {
  position: fixed;
  z-index: 20;
  right: 24px;
  bottom: 18px;
  left: 24px;
  display: flex;
  width: min(calc(100% - 48px), 540px);
  min-height: 76px;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin: 0 auto;
  padding: 11px 13px;
  border: 1px solid #e3e7e9;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 6px 28px rgba(26, 39, 55, 0.23);
  backdrop-filter: blur(12px);
}
.floating-copy {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 11px;
}
.floating-icon {
  display: inline-flex;
  width: 43px;
  height: 43px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  color: #ffffff;
  background: #20bf55;
}
.floating-icon :deep(svg) {
  width: 29px;
  height: 29px;
}
.floating-copy strong,
.floating-copy small {
  display: block;
}
.floating-copy strong {
  overflow: hidden;
  color: #101820;
  font-size: 16px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.floating-copy small {
  margin-top: 3px;
  color: #4f5a64;
  font-size: 12px;
}
.floating-button {
  display: inline-flex;
  min-width: 102px;
  min-height: 44px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  padding: 9px 15px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 850;
}
.header-contact:hover,
.primary-contact:hover,
.product-contact:hover,
.floating-button:hover {
  background: #129b40;
}
a:focus-visible,
button:focus-visible {
  outline: 3px solid #075895;
  outline-offset: 3px;
}

@media (max-width: 760px) {
  .landing-page { padding-bottom: 94px; }
  .wrap { padding-right: 24px; padding-left: 24px; }
  .header-inner { min-height: 86px; }
  .brand { gap: 9px; }
  .brand img { width: 36px; height: 36px; }
  .brand strong { max-width: 210px; font-size: 15px; }
  .brand small { font-size: 8px; letter-spacing: 2.5px; }
  .header-contact { min-height: 38px; padding: 7px 11px; font-size: 12px; }
  .hero {
    background:
      linear-gradient(90deg, rgba(237, 247, 252, 0.97) 0%, rgba(237, 247, 252, 0.78) 100%),
          url("/landing-assets/images/hero-79e77a51693b3249.webp") center / cover no-repeat;
  }
  .hero-content { padding-top: 42px; padding-bottom: 18px; }
  .eyebrow { font-size: 10px; letter-spacing: 1.7px; }
  h1 { margin-top: 20px; font-size: clamp(38px, 10.6vw, 48px); line-height: 0.99; letter-spacing: -1.8px; }
  .description { font-size: 14px; line-height: 1.75; }
  .reply-status { margin-top: 19px; font-size: 12px; }
  .primary-contact { min-height: 66px; gap: 13px; padding: 9px 18px; }
  .primary-contact :deep(svg) { width: 34px; height: 34px; }
  .primary-contact strong { font-size: 18px; }
  .primary-contact small { font-size: 12px; }
  .explore-link { margin-top: 18px; }
  .products-section { padding-top: 29px; padding-bottom: 38px; }
  .section-head { display: block; margin-bottom: 17px; }
  h2 { font-size: 25px; letter-spacing: -0.6px; }
  .section-head p { margin-top: 6px; font-size: 13px; }
  .catalog-link { display: inline-block; margin-top: 10px; font-size: 12px; }
  .products-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 13px; }
  .product-image { aspect-ratio: 1; }
  /* Compact text-first cards match the mobile ad layout and keep more products above the fold. */
  .product-image { display: none; }
  .badge { margin-bottom: 9px; padding: 4px 6px; font-size: 6.5px; }
  .product-copy { min-height: 164px; padding: 11px; }
  .product-copy h3 { font-size: 15px; }
  .product-copy p { margin: 7px 0 12px; font-size: 10px; line-height: 1.4; }
  .product-contact { min-height: 38px; gap: 5px; padding: 7px 8px; font-size: 12px; }
  .product-contact :deep(svg) { width: 18px; height: 18px; }
  .research-details { margin-bottom: 16px; padding-top: 28px; padding-bottom: 28px; }
  .floating-contact { right: 12px; bottom: 10px; left: 12px; width: calc(100% - 24px); min-height: 70px; padding: 9px 10px; border-radius: 16px; }
  .floating-icon { width: 40px; height: 40px; }
  .floating-copy { gap: 9px; }
  .floating-copy strong { max-width: 190px; font-size: 14px; }
  .floating-copy small { font-size: 11px; }
  .floating-button { min-width: 88px; min-height: 40px; padding: 8px 11px; font-size: 13px; }
}

@media (max-width: 390px) {
  .wrap { padding-right: 18px; padding-left: 18px; }
  .brand strong { max-width: 150px; }
  .floating-copy strong { max-width: 150px; }
}

@media (prefers-reduced-motion: reduce) {
  .product-image img { transition: none; }
}
</style>
