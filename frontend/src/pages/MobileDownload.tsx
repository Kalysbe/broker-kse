import { useState } from "react";
import { Link } from "react-router-dom";
import { AppPreview, MarketingIcon, PublicLayout } from "../components/Marketing";
import release from "../mobile-release.json";
import { detectMobileDevice } from "../mobileDevice";
import "../marketing.css";

export function DownloadCard() {
  const [device] = useState(detectMobileDevice);
  const [started, setStarted] = useState(false);
  const size = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(release.bytes / 1024 / 1024);
  return <div className="public-download-card"><span className="public-download-icon"><MarketingIcon name="phone" /></span><h3>КФБ Инвест</h3><p>Приложение для Android</p><div className="public-release-meta"><span>Версия {release.version}</span><span>Тестовая сборка</span></div>
    {device === "ios" ? <><p className="public-download-notice">На iPhone APK не устанавливается. Пользуйтесь веб-кабинетом — приложение для iOS пока недоступно.</p><Link className="public-button public-button-primary" to="/login">Открыть веб-кабинет <MarketingIcon name="arrow" /></Link></> : release.available ? <><a className="public-button public-button-primary" href={release.url} download="kfb-invest-android.apk" onClick={() => setStarted(true)}><MarketingIcon name="download" /> Скачать APK <span>{size} МБ</span></a><span className="public-download-requirements">{release.requirements} · без магазина приложений</span>{started && <p className="public-download-feedback" role="status">Откройте загруженный APK на Android. Если загрузка не началась, нажмите «Скачать APK» ещё раз.</p>}</> : <p className="public-download-notice" role="status">Сборка временно недоступна. Попробуйте позже или <Link to="/login">продолжите в веб-кабинете</Link>.</p>}
    <p className="public-test-note">Тестовая версия работает в сети брокера. Уточните доступ перед установкой.</p><Link className="public-install-link" to="/download#installation">Как установить приложение <MarketingIcon name="arrow" /></Link>
  </div>;
}

export function MobileDownloadPage() {
  return <PublicLayout><section className="public-container public-download-page"><div className="public-download-intro"><Link className="public-text-link" to="/welcome">← На главную</Link><div className="public-eyebrow">ПРИЛОЖЕНИЕ ВАШЕГО БРОКЕРА</div><h1>КФБ Инвест.<br /><span>Возьмите с собой.</span></h1><p className="public-lead">Ваш портфель, рынок ценных бумаг и заявки — в телефоне. Прямая загрузка с нашего сайта.</p><DownloadCard /></div><AppPreview compact /></section><section id="installation" className="public-container public-installation public-section"><div className="public-eyebrow">НЕСКОЛЬКО ПРОСТЫХ ШАГОВ</div><h2>Как установить на Android</h2><ol className="public-install-steps"><li><span>01</span><h3>Скачайте APK</h3><p>Откройте эту страницу на Android-телефоне и нажмите «Скачать APK». Файл появится в загрузках браузера.</p></li><li><span>02</span><h3>Откройте файл</h3><p>Нажмите на загруженный файл. Если Android запросит разрешение на установку из браузера, разрешите его для этой установки. После установки его можно отключить.</p></li><li><span>03</span><h3>Войдите в приложение</h3><p>Откройте КФБ Инвест и используйте логин и пароль своего брокера. Тестовой версии нужен доступ к сети брокера.</p></li></ol><div className="public-install-help"><MarketingIcon name="phone" /><p><strong>Пользуетесь iPhone?</strong> APK подходит только для Android. Откройте <Link to="/login">веб-кабинет</Link>, чтобы продолжить в браузере.</p></div>{release.available && <details className="public-file-details"><summary>Сведения о файле</summary><dl><dt>Файл</dt><dd>kfb-invest-android.apk</dd><dt>Версия</dt><dd>{release.version} · тестовая</dd><dt>Совместимость</dt><dd>{release.requirements}</dd><dt>SHA-256</dt><dd><code>{release.sha256}</code></dd></dl></details>}</section></PublicLayout>;
}
