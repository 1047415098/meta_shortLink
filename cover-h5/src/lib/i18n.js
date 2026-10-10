import { createI18n } from "vue-i18n";
import { coverWallPlaceholderTranslations } from "./coverWallPlaceholderTranslations.js";

const messages = Object.fromEntries(
  Object.entries(coverWallPlaceholderTranslations).map(([locale, copy]) => [locale, { openingStory: "Opening experience", ...copy }]),
);

export function createCoverI18n(bootstrap) {
  const available = bootstrap.available_locales || ["en"];
  const locale = available.includes(bootstrap.locale) && messages[bootstrap.locale] ? bootstrap.locale : "en";
  // The server resolves the first language from the visitor's IP and remembered cookie.
  document.documentElement.lang = locale;
  return createI18n({ legacy: false, locale, fallbackLocale: "en", messages });
}
