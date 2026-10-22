import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { ActivityIndicator, AppState, Modal, Platform, StyleSheet, Text, View } from 'react-native';
import Constants from 'expo-constants';
import * as Updates from 'expo-updates';
import { Button } from '../components';
import { useSession } from '../session';
import { colors, ui } from '../theme';
import { releaseInfo, type ReleaseInfo } from './release';

type Available = ReleaseInfo & { id: string };
type ContextValue = { check: () => Promise<void>; checking: boolean; version: string; notice: string };
const Context = createContext<ContextValue | null>(null);

export function AppUpdatesProvider({ children }: { children: ReactNode }) {
  const { user, initializing } = useSession();
  const [available, setAvailable] = useState<Available | null>(null);
  const [checking, setChecking] = useState(false);
  const [phase, setPhase] = useState<'available' | 'downloading' | 'restarting' | 'error'>('available');
  const [notice, setNotice] = useState('');
  const checkingRef = useRef(false), busy = useRef(false), lastCheck = useRef(0), dismissed = useRef('');
  const { downloadProgress } = Updates.useUpdates();
  const version = releaseInfo(Updates.manifest, Constants.nativeAppVersion || Constants.expoConfig?.version || '0.1.2').version;

  const check = useCallback(async (manual = false) => {
    if (checkingRef.current || busy.current) return;
    if (__DEV__ || Platform.OS === 'web' || !Updates.isEnabled) {
      if (manual) setNotice('Проверка обновлений доступна в установленном приложении.');
      return;
    }
    if (!manual && Date.now() - lastCheck.current < 60000) return;
    lastCheck.current = Date.now(); checkingRef.current = true; setChecking(true); setNotice('');
    try {
      const result = await Updates.checkForUpdateAsync();
      if (result.isAvailable) {
        const manifest = result.manifest;
        const id = 'id' in manifest ? String(manifest.id) : '';
        if (manual || id !== dismissed.current) {
          setAvailable({ id, ...releaseInfo(manifest, version) }); setPhase('available');
        }
      } else if (manual) setNotice('Установлена последняя версия.');
    } catch {
      // An unavailable update service must never prevent access to the broker.
      if (manual) setNotice('Не удалось проверить обновления. Попробуйте позже.');
    } finally { checkingRef.current = false; setChecking(false); }
  }, [version]);

  useEffect(() => {
    if (initializing) return;
    void check();
    const listener = AppState.addEventListener('change', state => {
      if (state === 'active' && !user) void check();
    });
    return () => listener.remove();
  }, [initializing, user?.id, check]);

  const later = () => {
    if (busy.current) return;
    dismissed.current = available?.id || ''; setAvailable(null); setPhase('available');
  };
  const install = async () => {
    if (busy.current) return;
    busy.current = true; setPhase('downloading');
    try {
      const result = await Updates.fetchUpdateAsync();
      if (!result.isNew && !result.isRollBackToEmbedded) { setAvailable(null); setNotice('Установлена последняя версия.'); return; }
      setPhase('restarting');
      await Updates.reloadAsync();
    } catch { setPhase('error'); }
    finally { busy.current = false; }
  };
  const working = phase === 'downloading' || phase === 'restarting';
  const progress = typeof downloadProgress === 'number' ? ` ${Math.round(Math.max(0, Math.min(1, downloadProgress)) * 100)}%` : '';

  return <Context.Provider value={{ check: () => check(true), checking, version, notice }}>{children}
    <Modal visible={!!available} transparent animationType="fade" onRequestClose={later}>
      <View style={s.overlay}><View style={s.dialog} accessibilityViewIsModal>
        <Text style={ui.title}>Доступно обновление</Text>
        <Text style={ui.label}>КФБ Инвест · {available?.version}</Text>
        {!!available?.message && <Text style={ui.subtitle}>{available.message}</Text>}
        <Text style={ui.subtitle}>Загрузим обновление прямо здесь. После установки приложение перезапустится.</Text>
        {working && <View style={s.progress}><ActivityIndicator color={colors.navy} /><Text accessibilityLiveRegion="polite" style={ui.body}>{phase === 'restarting' ? 'Применяем обновление…' : `Загружаем обновление…${progress}`}</Text></View>}
        {phase === 'error' && <Text accessibilityRole="alert" style={s.error}>Не удалось установить обновление. Текущая версия сохранена. Проверьте соединение и повторите.</Text>}
        <Button disabled={working} onPress={() => { void install(); }}>{phase === 'error' ? 'Повторить загрузку' : 'Обновить приложение'}</Button>
        <Button secondary disabled={working} onPress={later}>Позже</Button>
      </View></View>
    </Modal>
  </Context.Provider>;
}

export function useAppUpdates() {
  const context = useContext(Context);
  if (!context) throw new Error('AppUpdatesProvider missing');
  return context;
}

const s = StyleSheet.create({ overlay: { flex: 1, justifyContent: 'center', backgroundColor: 'rgba(15,23,42,0.45)', padding: 24 }, dialog: { width: '100%', maxWidth: 420, alignSelf: 'center', backgroundColor: colors.surface, borderRadius: 22, padding: 24, gap: 16 }, progress: { flexDirection: 'row', alignItems: 'center', gap: 12 }, error: { color: colors.red, fontSize: 13, lineHeight: 20 } });
