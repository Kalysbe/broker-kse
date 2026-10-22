import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";

const paths: Record<string, ReactNode> = {
  mark: <><path d="M5 19v-5m7 5V9m7 10V4" /></>,
  arrow: <><path d="M5 12h14m-6-6 6 6-6 6" /></>,
  download: <><path d="M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5" /></>,
  wallet: <><path d="M20 8H5a2 2 0 0 1 0-4h12v4M4 6v13a1 1 0 0 0 1 1h15V8m0 5h-5v3h5" /></>,
  market: <><path d="M4 20V4m0 16h16M8 15l4-5 4 2 5-7" /></>,
  orders: <><rect x="5" y="4" width="14" height="17" rx="2" /><path d="M9 3h6v4H9zM9 12h6m-6 4h4" /></>,
  shield: <><path d="m12 3 8 4v6c0 4-8 8-8 8s-8-4-8-8V7l8-4Z" /><path d="m8 12 3 3 5-6" /></>,
  check: <path d="m5 12 4 4L19 6" />,
  phone: <><rect x="6" y="2" width="12" height="20" rx="3" /><path d="M10 5h4m-3 14h2" /></>,
  close: <path d="m6 6 12 12M6 18 18 6" />,
  menu: <path d="M4 6h16M4 12h16M4 18h16" />,
  eye: <><path d="M3 3l18 18M10 5c5-1 9 3 11 7a17 17 0 0 1-3 4M6 6a20 20 0 0 0-5 6c3 6 8 9 14 6" /><path d="M10 10a3 3 0 0 0 4 4" /></>,
};

export function MarketingIcon({ name, className = "" }: { name: string; className?: string }) {
  return <svg className={className} width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name] || paths.mark}</svg>;
}

export function PublicBrand() {
  return <Link className="public-brand" to="/welcome" aria-label="КФБ Инвест — главная"><span className="public-brand-mark"><MarketingIcon name="mark" /></span><span>КФБ <strong>Инвест</strong><small>Инвестиции в вашем ритме</small></span></Link>;
}

export function PublicHeader() {
  const [menuOpen, setMenuOpen] = useState(false);
  return <header className="public-header"><div className="public-container public-header-inner">
    <PublicBrand />
    <nav className={`public-nav ${menuOpen ? "is-open" : ""}`} id="public-nav" aria-label="Основная навигация" onClick={() => setMenuOpen(false)}>
      <Link to="/welcome#features">Возможности</Link><Link to="/welcome#how-it-works">Как это работает</Link><Link to="/welcome#questions">Вопросы</Link>
    </nav>
    <div className="public-header-actions"><Link className="public-login" to="/login">Веб-кабинет <MarketingIcon name="arrow" /></Link><button className="public-menu-button" aria-label={menuOpen ? "Закрыть меню" : "Открыть меню"} aria-expanded={menuOpen} aria-controls="public-nav" onClick={() => setMenuOpen(!menuOpen)}><MarketingIcon name={menuOpen ? "close" : "menu"} /></button></div>
  </div></header>;
}

export function PublicFooter() {
  return <footer className="public-footer"><div className="public-container"><div className="public-footer-top"><PublicBrand /><div><Link to="/download">Скачать приложение</Link><Link to="/login">Войти в веб-кабинет</Link></div></div><div className="public-footer-bottom"><span>© {new Date().getFullYear()} КФБ Инвест</span><p>Стоимость ценных бумаг может расти и снижаться. Инвестиционные решения остаются за вами.</p></div></div></footer>;
}

export function PublicLayout({ children }: { children: ReactNode }) {
  const container = useRef<HTMLDivElement>(null);
  const { pathname, hash } = useLocation();
  useEffect(() => {
    if (hash) document.getElementById(hash.slice(1))?.scrollIntoView();
    else container.current?.scrollTo({ top: 0, behavior: "instant" });
  }, [pathname, hash]);
  return <div className="public-site" ref={container}><a className="public-skip" href="#public-content">Перейти к содержимому</a><PublicHeader /><main id="public-content" tabIndex={-1}>{children}</main><PublicFooter /></div>;
}

type Preview = "portfolio" | "market" | "orders";
export function AppPreview({ compact = false }: { compact?: boolean }) {
  const [preview, setPreview] = useState<Preview>("portfolio");
  const tabs = [{ id: "portfolio", label: "Портфель", icon: "wallet" }, { id: "market", label: "Рынок", icon: "market" }, { id: "orders", label: "Заявки", icon: "orders" }] as const;
  return <div className={`app-showcase ${compact ? "is-compact" : ""}`}>
    <div className="app-orbit app-orbit-one" aria-hidden="true" /><div className="app-orbit app-orbit-two" aria-hidden="true" />
    <div className="showcase-note"><span className="showcase-note-icon"><MarketingIcon name="shield" /></span><div>Ваш счёт.<br /><strong>Ваш контроль.</strong></div></div>
    <div className="preview-phone" role="img" aria-label={`Пример интерфейса КФБ Инвест: ${tabs.find(tab => tab.id === preview)?.label}. Суммы скрыты, реальные данные не используются.`}>
      <div className="phone-camera" /><div className="phone-status"><b>9:41</b><span>▮▮▮ &nbsp; ▰</span></div>
      <div className="phone-body"><div className="phone-brand"><span><MarketingIcon name="mark" /></span>КФБ Инвест<div className="phone-avatar">К</div></div>
        <div className="phone-greeting">Всё важное — перед вами</div><h3>{preview === "portfolio" ? "Мой портфель" : preview === "market" ? "Обзор рынка" : "Мои заявки"}</h3>
        {preview === "portfolio" && <><div className="phone-balance"><div>Деньги на счёте<MarketingIcon name="eye" /></div><strong>••• ••• <small>с</small></strong><span>Суммы скрыты</span><div className="phone-balance-line" /><div>Зарезервировано <b>••• •••</b></div></div><div className="phone-section-title">Ценные бумаги <span>Обзор</span></div><div className="phone-instrument"><span className="phone-instrument-icon">А</span><div><b>Акции</b><small>Доля в компаниях</small></div><span>•••</span></div><div className="phone-instrument"><span className="phone-instrument-icon bond">О</span><div><b>Облигации</b><small>Долговые ценные бумаги</small></div><span>•••</span></div><div className="phone-history"><MarketingIcon name="orders" /> История операций <span>›</span></div></>}
        {preview === "market" && <><div className="phone-search">Поиск инструментов <span>⌕</span></div><div className="phone-chips"><b>Все</b><span>Акции</span><span>Облигации</span></div><div className="phone-section-title">Инструменты <span>КФБ</span></div><div className="phone-instrument"><span className="phone-instrument-icon">А</span><div><b>Акции</b><small>Участие в бизнесе</small></div><span>›</span></div><div className="phone-instrument"><span className="phone-instrument-icon bond">О</span><div><b>Облигации</b><small>Выбор по параметрам</small></div><span>›</span></div><div className="phone-info"><MarketingIcon name="market" /><b>Соберите своё избранное</b><p>Следите за интересующими вас инструментами.</p></div></>}
        {preview === "orders" && <><div className="phone-chips"><b>Все заявки</b><span>Активные</span></div><div className="phone-order-example"><span className="phone-demo-label">Пример проверки заявки</span><h4>Всё готово к проверке</h4><div>Инструмент <b>Выбираете вы</b></div><div>Цена и количество <b>Проверьте детали</b></div><div>Отправка <b>После подтверждения</b></div><span className="phone-confirm"><MarketingIcon name="check" /> Решение за вами</span></div><div className="phone-info"><MarketingIcon name="orders" /><b>Следите за статусом</b><p>Заявки и история в одном разделе.</p></div></>}
      </div>
      <div className="phone-bottom-nav">{[{ icon: "mark", label: "Главная" }, { icon: "market", label: "Рынок" }, { icon: "wallet", label: "Портфель" }, { icon: "orders", label: "Заявки" }].map(t => <span key={t.label} className={t.label === tabs.find(tab => tab.id === preview)?.label ? "active" : ""}><MarketingIcon name={t.icon} />{t.label}</span>)}<span><span className="phone-profile-icon">○</span>Профиль</span></div><div className="phone-home-bar" />
    </div>
    <div className="preview-switcher" aria-label="Посмотреть экраны приложения">{tabs.map(tab => <button key={tab.id} type="button" aria-pressed={preview === tab.id} onClick={() => setPreview(tab.id)}>{tab.label}</button>)}</div>
    <p className="preview-caption">Знакомство с интерфейсом · без данных счёта</p>
  </div>;
}
