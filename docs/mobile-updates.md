# Обновления из приложения

APK 0.1.2 (versionCode 3) добавляет проверку при открытии и входе, предложение
«Обновить приложение / Позже», загрузку с индикатором и перезапуск. Сессия
сохраняется в SecureStore. В профиле есть номер версии и ручная проверка.
Недоступность сервера обновлений не блокирует вход в брокерское приложение.

APK 0.1.1 и старше нужно один раз заменить новым APK. Затем интерфейс,
JavaScript и ресурсы можно обновлять без APK. Новые нативные библиотеки,
разрешения Android, Expo SDK или сертификат подписи требуют нового APK и нового
`runtimeVersion`. Текущий runtime — `sbis-android-ota-1`.

## Устройство сервиса

Обновления обслуживает наш Go API без EAS и учетной записи Expo:

- `GET /api/v1/mobile/updates`: выбор по `expo-platform` и
  `expo-runtime-version`. Ответ `204` означает отсутствие совместимого обновления
  или то, что текущий выпуск уже установлен.
- `GET /api/v1/mobile/assets/{sha256}.{ext}`: файлы с Content-Length,
  поддержкой Range и длительным кешированием по неизменяемому хешу.

Данные сервера: `/home/nysh/broker-kse/mobile-updates`, папки `assets`,
`releases` и `current/{runtime}/android.json`. API монтирует их только для чтения;
содержимое исключено из Git.

Манифест подписывается локально RSA-SHA256. Приложение проверяет подпись по
`mobile/certs/certificate.pem` и SHA-256 всех файлов. Закрытый ключ хранится в
`.deploy/update-signing/private-key.pem`; сохраните его в резервной копии
материалов выпуска. Он не входит в Git, APK или архив обновления и не нужен на
сервере. Защитные механизмы expo-updates остаются включенными: при ошибке запуска
модуль может вернуться к рабочей или встроенной версии.

## Выпуск обновления

Для обычного изменения JavaScript не меняйте `runtimeVersion`, versionCode и
версию базового APK. Номер обновления задается отдельно:

```powershell
cd mobile
npm test
npm run typecheck
npm run update:prepare -- --version 0.1.3 --message "Улучшена работа приложения."
```

Команда экспортирует Android/Hermes, считает хеши, проверяет ключ и готовит
`.deploy/mobile-update-release`. Адрес API по умолчанию `http://172.23.10.97`;
для другого совместимого контура задайте `EXPO_PUBLIC_API_URL` перед сборкой.

Из корня проекта архивируйте **только** подготовленный выпуск, а не всю `.deploy`:

```powershell
tar -czf .deploy/mobile-update.tar.gz -C .deploy/mobile-update-release assets releases current
scp .deploy/mobile-update.tar.gz nysh@172.23.10.97:mobile-update.tar.gz
```

На сервере сначала скопируйте файлы и историю, затем атомарно замените текущий
манифест. Перезапуск API не требуется:

```bash
cd /home/nysh/broker-kse
stage=$(mktemp -d /home/nysh/broker-kse-update.XXXXXX)
tar -xzf /home/nysh/mobile-update.tar.gz -C "$stage" --no-same-owner
mkdir -p mobile-updates/assets mobile-updates/releases mobile-updates/current/sbis-android-ota-1
cp -n "$stage"/assets/* mobile-updates/assets/
cp "$stage"/releases/*.json mobile-updates/releases/
cp "$stage"/current/sbis-android-ota-1/android.json mobile-updates/current/sbis-android-ota-1/android.json.next
mv mobile-updates/current/sbis-android-ota-1/android.json.next mobile-updates/current/sbis-android-ota-1/android.json
```

Проверьте предложение на устройстве, откладывание, загрузку, перезапуск,
сохранение сессии и номер версии в профиле. Не удаляйте предыдущие файлы сразу:
они нужны устройствам, уже начавшим загрузку. Для отката экспортируйте проверенный
прежний код и выпустите его заново с новым ID и временем создания. Простое
возвращение старого манифеста не гарантирует откат обновленных устройств.

Спецификация: [Expo Updates v1](https://docs.expo.dev/technical-specs/expo-updates-1/).
Совместимость: [Runtime versions](https://docs.expo.dev/eas-update/runtime-versions/).
