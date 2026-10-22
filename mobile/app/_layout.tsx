import { useEffect } from 'react';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { AppState, Platform, StyleSheet, Text, View } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query';
import { SessionProvider, useSession } from '../src/session';
import { BrokerProvider } from '../src/broker';
import { Button, Card, Page } from '../src/components';
import { errorMessage } from '../src/api/client';
import { colors } from '../src/theme';

export default function RootLayout() {
  return <SafeAreaProvider><QueryClientProvider client={queryClient}><SessionProvider><BrokerProvider><ApplicationFrame /></BrokerProvider></SessionProvider></QueryClientProvider></SafeAreaProvider>;
}
function ApplicationFrame() {
  const { user } = useSession();
  return <View style={s.desktop}><SafeAreaView style={[s.app, !user && { backgroundColor: colors.surface }]} edges={['top', 'left', 'right']}><StatusBar style="dark" /><Navigation /></SafeAreaView></View>;
}
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 15000 }, mutations: { retry: false } } });
function Navigation() {
  const { user, initializing, startupError, retry, logout } = useSession();
  useEffect(() => { const listener = AppState.addEventListener('change', state => focusManager.setFocused(state === 'active')); return () => listener.remove(); }, []);
  if (initializing) return <Page><Card><Text>Загружаем…</Text></Card></Page>;
  if (startupError) return <Page title="Нет соединения"><Card style={{ gap: 18 }}><Text accessibilityRole="alert">{errorMessage(startupError)}</Text><Button onPress={retry}>Повторить</Button><Button secondary onPress={() => { void logout(); }}>Перейти ко входу</Button></Card></Page>;
  return <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.background }, animation: 'slide_from_right' }}>
    <Stack.Protected guard={!user}><Stack.Screen name="login" /></Stack.Protected>
    <Stack.Protected guard={!!user}>
      <Stack.Screen name="(tabs)" /><Stack.Screen name="trade" options={{ gestureEnabled: false }} />
      <Stack.Screen name="instrument/[id]" /><Stack.Screen name="history" /><Stack.Screen name="notifications" /><Stack.Screen name="settings" /><Stack.Screen name="account" /><Stack.Screen name="transfer" /><Stack.Screen name="identification" />
    </Stack.Protected>
    <Stack.Screen name="+not-found" />
  </Stack>;
}
const s = StyleSheet.create({ desktop: { flex: 1, backgroundColor: '#E7ECF2', alignItems: 'center' }, app: { flex: 1, width: '100%', maxWidth: Platform.OS === 'web' ? 402 : undefined, backgroundColor: colors.background } });

