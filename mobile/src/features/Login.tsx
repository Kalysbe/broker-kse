import { useState } from 'react';
import { KeyboardAvoidingView, Platform, StyleSheet, Text, TextInput, View } from 'react-native';
import { ChartNoAxesColumnIncreasing, ShieldCheck, TrendingUp } from 'lucide-react-native';
import Svg, { Path } from 'react-native-svg';
import { Button, Card, Notice, Page } from '../components';
import { ApiError, errorMessage } from '../api/client';
import { useSession } from '../session';
import { colors, ui } from '../theme';

export default function LoginScreen() {
  const { login: signIn } = useSession();
  const [screen, setScreen] = useState<'welcome' | 'login' | 'registration'>('welcome');
  const [login, setLogin] = useState(''), [password, setPassword] = useState(''), [totp, setTotp] = useState('');
  const [needsTotp, setNeedsTotp] = useState(false), [pending, setPending] = useState(false), [error, setError] = useState('');
  const submit = async () => {
    if (pending || !login.trim() || !password) return;
    setPending(true); setError('');
    try { await signIn(login, password, totp); setPassword(''); setTotp(''); }
    catch (problem) { if (problem instanceof ApiError && problem.code === 'TOTP_REQUIRED') setNeedsTotp(true); setError(errorMessage(problem)); }
    finally { setPending(false); }
  };
  if (screen === 'welcome') return <Page surface contentStyle={s.welcomeContent}>
    <View style={s.brand}><View style={s.brandIcon}><ChartNoAxesColumnIncreasing color="#FFF" size={23} strokeWidth={1.8} /></View><Text style={s.wordmark}>КФБ Инвест</Text></View>
    <View style={s.hero} accessible accessibilityLabel="Акции и облигации Кыргызстана"><View style={s.illustration}><Text style={s.chartCaption}>Ваш первый шаг к инвестициям</Text><Svg width={270} height={100} viewBox="0 0 270 100" style={s.chart}><Path d="M6 85 C20 84 28 78 41 75 S65 76 77 79 S95 70 108 50 S123 43 135 48 S151 60 164 40 S183 28 198 34 S216 29 231 28 S251 26 264 14" fill="none" stroke={colors.ink} strokeWidth={2.2} strokeLinecap="round" /></Svg><View style={s.chartLegend}><TrendingUp size={14} color={colors.navy} /><Text style={s.chartLegendText}>Акции и облигации Кыргызстана</Text></View></View><View style={s.shield}><ShieldCheck size={25} strokeWidth={1.4} color="#FFF" /></View></View>
    <View style={s.heading}><Text style={s.title}>Инвестируйте{`\n`}в то, что знаете</Text><Text style={s.subtitle}>Откройте для себя ценные бумаги на Кыргызской фондовой бирже. Всё понятно — от выбора до первой заявки.</Text></View>
    <View style={s.actions}><Button onPress={() => setScreen('registration')}>Регистрация</Button><Button secondary onPress={() => setScreen('login')}>Войти</Button></View>
    <Text style={s.risk}>Стоимость инвестиций может расти и снижаться.</Text>
  </Page>;
  if (screen === 'registration') return <Page surface title="Открытие счёта" back onBack={() => setScreen('welcome')}><Notice title="Счёт у вашего брокера">Для открытия брокерского счёта и подтверждения личности обратитесь к вашему брокеру. После оформления договора вы сможете войти в приложение.</Notice><Button onPress={() => setScreen('login')}>У меня уже есть счёт</Button></Page>;
  return <KeyboardAvoidingView style={s.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}><Page surface title="Вход" back onBack={() => { setPassword(''); setTotp(''); setError(''); setScreen('welcome'); }}>
    <Text style={s.subtitle}>Войдите в свой брокерский счёт</Text>
    <Card style={s.form}>
      <View style={s.field}><Text style={ui.label}>Логин</Text><TextInput accessibilityLabel="Логин" autoCapitalize="none" autoCorrect={false} autoComplete="username" textContentType="username" editable={!pending} value={login} onChangeText={setLogin} style={ui.input} returnKeyType="next" /></View>
      <View style={s.field}><Text style={ui.label}>Пароль</Text><TextInput accessibilityLabel="Пароль" secureTextEntry autoCapitalize="none" autoCorrect={false} autoComplete="current-password" textContentType="password" editable={!pending} value={password} onChangeText={setPassword} style={ui.input} returnKeyType="go" onSubmitEditing={() => { void submit(); }} /></View>
      {needsTotp && <View style={s.field}><Text style={ui.label}>Код аутентификатора</Text><TextInput accessibilityLabel="Код аутентификатора" keyboardType="number-pad" autoComplete="one-time-code" editable={!pending} value={totp} onChangeText={value => setTotp(value.replace(/\D/g, '').slice(0, 6))} style={ui.input} maxLength={6} /></View>}
      {!!error && <Text accessibilityRole="alert" style={s.error}>{error}</Text>}
      <Button disabled={pending || !login.trim() || !password || needsTotp && totp.length !== 6} onPress={() => { void submit(); }}>{pending ? 'Входим…' : 'Войти'}</Button>
    </Card>
    <View style={s.security}><ShieldCheck size={18} strokeWidth={1.5} color={colors.muted} /><Text style={s.securityText}>Ваши данные защищены</Text></View>
    <Text style={s.help}>Для восстановления доступа обратитесь к вашему брокеру.</Text>
  </Page></KeyboardAvoidingView>;
}
const s = StyleSheet.create({
  welcomeContent: { paddingHorizontal: 28, paddingTop: 28 }, flex: { flex: 1, backgroundColor: '#FFF' }, brand: { flexDirection: 'row', alignItems: 'center', gap: 11, paddingTop: 20, marginBottom: 6 }, brandIcon: { width: 40, height: 40, backgroundColor: colors.navy, borderRadius: 12, alignItems: 'center', justifyContent: 'center' }, wordmark: { color: colors.ink, fontSize: 23, fontWeight: '600', letterSpacing: -0.5 },
  hero: { minHeight: 286, backgroundColor: colors.background, borderRadius: 30, alignItems: 'center', justifyContent: 'center', padding: 30, marginTop: 7 }, illustration: { width: '100%', backgroundColor: '#FFF', borderRadius: 18, padding: 18, paddingBottom: 20 }, chartCaption: { color: colors.muted, fontSize: 11, marginBottom: 9 }, chart: { alignSelf: 'center', maxWidth: '100%' }, chartLegend: { flexDirection: 'row', gap: 6, alignItems: 'center', marginTop: 5 }, chartLegendText: { color: colors.ink, fontSize: 10, fontWeight: '500' }, shield: { width: 58, height: 58, borderRadius: 17, backgroundColor: colors.navy, position: 'absolute', right: 31, bottom: 33, alignItems: 'center', justifyContent: 'center' },
  heading: { gap: 14, paddingTop: 14, paddingBottom: 7 }, title: { fontSize: 32, lineHeight: 43, color: colors.ink, fontWeight: '600', letterSpacing: -0.8 }, subtitle: { color: colors.muted, fontSize: 14, lineHeight: 22 }, actions: { gap: 12 }, risk: { textAlign: 'center', color: colors.muted, fontSize: 11, lineHeight: 18, paddingHorizontal: 18, marginTop: 14 },
  form: { gap: 21, paddingHorizontal: 0 }, field: { gap: 10 }, error: { fontSize: 13, lineHeight: 20, color: colors.red }, security: { flexDirection: 'row', justifyContent: 'center', alignItems: 'center', gap: 9, paddingTop: 16 }, securityText: { color: colors.muted, fontSize: 12, lineHeight: 19 }, help: { color: colors.muted, fontSize: 12, lineHeight: 20, textAlign: 'center', paddingHorizontal: 20, marginTop: 12 },
});
