import { useState } from 'react';
import { StyleSheet, Switch, Text, View } from 'react-native';
import { router } from 'expo-router';
import { Bell, Building2, FileText, History, LogOut, Settings2, ShieldCheck, UserRound } from 'lucide-react-native';
import { Button, Card, DetailRow, Empty, MenuRow, Notice, Page } from '../components';
import { useAccounts, useNotifications, usePreferences } from '../broker';
import { useSession } from '../session';
import { colors, ui } from '../theme';
import { AccountFeedback, AccountPicker, NoAccess, QueryFeedback, dateText, liveStyles as s } from './common';

export function ProfileScreen() {
  const { user, logout } = useSession();
  const [confirmExit, setConfirmExit] = useState(false), [pending, setPending] = useState(false);
  return <Page title="Профиль"><Card><View style={p.identity}><View style={p.avatar}><Text style={p.initials}>{user?.first_name.slice(0, 1)}{user?.last_name.slice(0, 1)}</Text></View><View style={s.flex}><Text style={p.name}>{[user?.first_name, user?.last_name].filter(Boolean).join(' ') || user?.login}</Text><Text style={ui.caption}>{user?.phone || user?.email || user?.login}</Text></View></View></Card>
    <Text style={p.groupTitle}>ДАННЫЕ И СЧЁТ</Text><Card style={s.listCard}><MenuRow icon={UserRound} title="Личные данные" detail="Контактная информация и счёт" onPress={() => router.push('/account')} /><MenuRow icon={FileText} title="Мои документы" onPress={() => router.push('/identification')} /><MenuRow icon={Building2} title="Банковский счёт" onPress={() => router.push('/transfer')} last /></Card>
    <Text style={p.groupTitle}>НАСТРОЙКИ</Text><Card style={s.listCard}><MenuRow icon={ShieldCheck} title="Безопасность" detail="Настройки доступа" onPress={() => router.push('/settings')} /><MenuRow icon={Bell} title="Уведомления" detail="Сообщения брокера" onPress={() => router.push('/notifications')} last /></Card>
    <Button secondary icon={LogOut} onPress={() => setConfirmExit(true)}>Выйти из аккаунта</Button>
    {confirmExit && <Card style={p.logout}><Text style={ui.label}>Выйти из аккаунта?</Text><Text style={ui.subtitle}>Для следующего входа понадобится пароль.</Text><Button disabled={pending} onPress={() => { setPending(true); void logout().finally(() => setPending(false)); }}>Выйти</Button><Button secondary disabled={pending} onPress={() => setConfirmExit(false)}>Остаться</Button></Card>}
  </Page>;
}
export function SettingsScreen() {
  const prefs = usePreferences(), { logout } = useSession();
  const [confirm, setConfirm] = useState(false), [pending, setPending] = useState(false);
  return <Page title="Настройки" back><Card><View style={ui.between}><View style={s.flex}><Text style={ui.label}>Скрывать суммы</Text><Text style={ui.caption}>На главной и в портфеле</Text></View><Switch accessibilityLabel="Скрывать суммы" value={prefs.hiddenBalance} onValueChange={prefs.toggleBalance} trackColor={{ false: '#D7DEE7', true: colors.navy }} thumbColor="#FFF" /></View></Card><Card style={p.logout}><Text style={ui.label}>Выход из учётной записи</Text><Text style={ui.subtitle}>Данные счетов будут удалены из памяти приложения. Для следующего входа понадобится пароль.</Text><Button secondary icon={LogOut} onPress={() => setConfirm(true)}>Выйти из приложения</Button>{confirm && <><Button disabled={pending} onPress={() => { setPending(true); void logout().finally(() => setPending(false)); }}>Подтвердить выход</Button><Button secondary disabled={pending} onPress={() => setConfirm(false)}>Остаться в приложении</Button></>}</Card></Page>;
}
export function AccountScreen() {
  const accounts = useAccounts(), { user } = useSession();
  return <Page title="Информация о счёте" back><AccountFeedback query={accounts} />{accounts.account && <><AccountPicker {...accounts} onSelect={accounts.selectAccount} /><Card><DetailRow label="Номер счёта" value={accounts.account.account_number} /><DetailRow label="Валюта" value={accounts.account.currency} /><DetailRow label="Статус" value={accounts.account.status === 'ACTIVE' ? 'Активен' : accounts.account.status} /><DetailRow label="Пользователь" value={[user?.first_name, user?.last_name].filter(Boolean).join(' ') || user?.login || '—'} /></Card></>}</Page>;
}
export function NotificationsScreen() {
  const query = useNotifications(), { can } = useSession();
  return <Page title="Уведомления" back>{!can('notifications.manage') ? <NoAccess /> : <><QueryFeedback query={query} />{query.data && (query.data.items.length ? query.data.items.map(item => <Card key={item.id}><Text style={ui.label}>{item.title}</Text><Text style={p.notificationBody}>{item.body}</Text><Text style={ui.caption}>{dateText(item.created_at)}</Text></Card>) : <Empty title="Новых уведомлений нет" detail="Здесь появятся сообщения от брокера." />)}</>}</Page>;
}
export function TransferScreen() { return <Page title="Переводы" back><Notice title="Реквизиты брокера">Для пополнения или вывода средств обратитесь к вашему брокеру. Банковские реквизиты и условия перевода уточняются по договору.</Notice><Button onPress={() => router.replace('/portfolio')}>В портфель</Button></Page>; }
export function IdentificationScreen() { return <Page title="Идентификация" back><Notice>Для открытия счёта и прохождения идентификации обратитесь к вашему брокеру. Документы загружаются только через согласованный с брокером канал.</Notice></Page>; }
const p = StyleSheet.create({ groupTitle: { color: colors.muted, fontSize: 10, fontWeight: '500', letterSpacing: 0.8, marginTop: 8, marginBottom: -7 }, name: { color: colors.ink, fontSize: 17, fontWeight: '600', marginBottom: 5 }, identity: { flexDirection: 'row', alignItems: 'center', gap: 14 }, avatar: { width: 58, height: 58, borderRadius: 20, backgroundColor: colors.soft, alignItems: 'center', justifyContent: 'center' }, initials: { fontSize: 20, fontWeight: '600', color: colors.navy }, note: { color: colors.muted, fontSize: 12, lineHeight: 20 }, logout: { gap: 17 }, notificationBody: { color: colors.ink, fontSize: 14, lineHeight: 21, marginVertical: 14 } });
