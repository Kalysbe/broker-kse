param([string]$Device)
$ErrorActionPreference = 'Stop'
$mobileRoot = Split-Path -Parent $PSScriptRoot
$apk = Join-Path $mobileRoot 'artifacts\kfb-invest-android.apk'
$adb = Join-Path $env:LOCALAPPDATA 'Android\Sdk\platform-tools\adb.exe'
if (!(Test-Path -LiteralPath $apk)) { throw 'Сначала соберите APK: node scripts/android.mjs phone' }
if (!(Test-Path -LiteralPath $adb)) { throw 'Не найден Android SDK platform-tools.' }
$connected = @(& $adb devices | ForEach-Object {
    if ($_ -match '^(\S+)\s+device$' -and $Matches[1] -notlike 'emulator-*') { $Matches[1] }
})
if (!$Device) {
    if ($connected.Count -ne 1) { throw 'Подключите один Android-телефон по USB, включите отладку по USB и подтвердите доверие компьютеру. Для нескольких телефонов используйте -Device SERIAL.' }
    $Device = $connected[0]
}
if ($Device -notin $connected) { throw 'Указанный телефон не подключён или не подтвердил отладку по USB.' }
& $adb -s $Device install -r $apk
if ($LASTEXITCODE -ne 0) { throw 'Не удалось установить APK.' }
& $adb -s $Device shell am start -n kg.sbis.mobile.localphone/kg.sbis.mobile.MainActivity
if ($LASTEXITCODE -ne 0) { throw 'Не удалось запустить приложение.' }
Write-Host 'КФБ Инвест установлен. USB можно отключить. В стандартной сборке API: http://172.23.10.97; телефон должен иметь доступ к сети сервера. Metro не нужен.'
