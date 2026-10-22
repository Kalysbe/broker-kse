import { router } from 'expo-router';
import { Empty, Page } from '../src/components';
export default function NotFound() { return <Page title="Страница не найдена"><Empty title="Здесь пока ничего нет" detail="Вернитесь на главную, чтобы продолжить работу с демосчётом." action="На главную" onPress={() => router.replace('/')} /></Page>; }
