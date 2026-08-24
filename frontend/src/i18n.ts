import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./locales/en.json";
import ru from "./locales/ru.json";
import ky from "./locales/ky.json";

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, ru: { translation: ru }, ky: { translation: ky } },
  lng: localStorage.getItem("lang") || "ru",
  fallbackLng: "ru",
  interpolation: { escapeValue: false },
});

export default i18n;
