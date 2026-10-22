import { useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { detectMobileDevice } from "../mobileDevice";
import { MarketingIcon } from "./Marketing";
import release from "../mobile-release.json";
import { useAuth } from "../auth";
import "../marketing.css";

const dismissalKey = "kfb-mobile-app-dismissed";

export function MobileAppPrompt() {
  const { pathname } = useLocation();
  const { authenticated } = useAuth();
  const dialog = useRef<HTMLDialogElement>(null);
  const [device] = useState(detectMobileDevice);
  const [dismissed, setDismissed] = useState(() => {
    try { return sessionStorage.getItem(dismissalKey) === "1"; } catch { return false; }
  });
  const isCabinet = !["/", "/welcome", "/download"].includes(pathname.replace(/\/$/, "") || "/");
  // The dashboard also lives at /; show the offer there only after sign-in.
  const authenticatedRoot = pathname === "/" && authenticated;
  const shouldOpen = device !== "desktop" && !dismissed && (isCabinet || authenticatedRoot);

  useEffect(() => {
    const element = dialog.current;
    if (!shouldOpen || !element) return;
    element.showModal();
    return () => element.close();
  }, [shouldOpen]);

  function dismiss() {
    try { sessionStorage.setItem(dismissalKey, "1"); } catch { /* The in-memory choice still lasts for this visit. */ }
    setDismissed(true);
  }

  return <dialog ref={dialog} className="mobile-app-dialog" aria-labelledby="mobile-app-title" aria-describedby="mobile-app-description" onCancel={event => { event.preventDefault(); dismiss(); }}>
    <button className="mobile-app-close" aria-label="Закрыть предложение" onClick={dismiss}><MarketingIcon name="close" /></button><span className="public-download-icon"><MarketingIcon name="phone" /></span><div className="public-eyebrow">КФБ ИНВЕСТ</div><h2 id="mobile-app-title">С телефона удобнее<br />с приложением</h2><p id="mobile-app-description">{device === "ios" ? "APK доступен только для Android. На iPhone вы можете продолжить работу в веб-кабинете." : "Портфель, рынок и заявки — под рукой. Скачайте приложение прямо с нашего сайта."}</p>
    {device !== "ios" && release.available && <><Link className="public-button public-button-primary" to="/download" onClick={dismiss}><MarketingIcon name="download" /> Скачать для Android</Link><p className="mobile-app-small">Тестовая версия · APK · работает в сети брокера</p></>}
    {device !== "ios" && !release.available && <p className="mobile-app-small">Сборка временно недоступна для скачивания.</p>}
    <button className="public-button public-button-secondary" onClick={dismiss}>Продолжить в браузере <MarketingIcon name="arrow" /></button>
  </dialog>;
}
