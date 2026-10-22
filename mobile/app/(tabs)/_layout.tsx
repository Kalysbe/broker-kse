import { Tabs } from 'expo-router';
import { ChartNoAxesCombined, House, BriefcaseBusiness, ClipboardList, UserRound } from 'lucide-react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors } from '../../src/theme';

export default function TabLayout() {
  const insets = useSafeAreaInsets();
  return <Tabs screenOptions={{ headerShown: false, tabBarActiveTintColor: colors.navy, tabBarInactiveTintColor: '#8C96A4', tabBarStyle: { backgroundColor: '#FFF', borderTopColor: colors.line, height: 64 + Math.max(12, insets.bottom), paddingTop: 10, paddingBottom: Math.max(12, insets.bottom) }, tabBarLabelStyle: { fontSize: 10, marginTop: 4 }, sceneStyle: { backgroundColor: colors.background } }}>
    <Tabs.Screen name="index" options={{ title: 'Главная', tabBarIcon: ({ color }) => <House size={21} strokeWidth={1.6} color={color} /> }} />
    <Tabs.Screen name="market" options={{ title: 'Рынок', tabBarIcon: ({ color }) => <ChartNoAxesCombined size={21} strokeWidth={1.6} color={color} /> }} />
    <Tabs.Screen name="portfolio" options={{ title: 'Портфель', tabBarIcon: ({ color }) => <BriefcaseBusiness size={21} strokeWidth={1.6} color={color} /> }} />
    <Tabs.Screen name="orders" options={{ title: 'Заявки', tabBarIcon: ({ color }) => <ClipboardList size={21} strokeWidth={1.6} color={color} /> }} />
    <Tabs.Screen name="profile" options={{ title: 'Профиль', tabBarIcon: ({ color }) => <UserRound size={21} strokeWidth={1.6} color={color} /> }} />
  </Tabs>;
}

