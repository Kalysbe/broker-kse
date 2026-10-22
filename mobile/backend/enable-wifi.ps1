$ErrorActionPreference = 'Stop'
$statusPath = Join-Path (Split-Path -Parent $PSScriptRoot) '.dev-data\wifi-firewall.json'
try {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    if (!([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Для добавления правила Windows Firewall нужны права администратора.'
    }
    $ruleName = 'SBIS-Mobile-API-WiFi'
    $existing = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
    if ($existing) {
        Set-NetFirewallRule -Name $ruleName -Enabled True -Direction Inbound -Action Allow -Profile Any -InterfaceAlias 'Ethernet 2' -EdgeTraversalPolicy Block | Out-Null
        Get-NetFirewallRule -Name $ruleName | Get-NetFirewallAddressFilter | Set-NetFirewallAddressFilter -LocalAddress '192.168.20.96' -RemoteAddress '192.168.20.0/24' | Out-Null
        Get-NetFirewallRule -Name $ruleName | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort 18080 -RemotePort Any | Out-Null
    } else {
        New-NetFirewallRule -Name $ruleName -DisplayName 'KFB Invest - local API 18080 (Wi-Fi)' -Description 'Development broker API: 192.168.20.96:18080, local subnet only.' -Enabled True -Direction Inbound -Action Allow -Protocol TCP -LocalPort 18080 -LocalAddress '192.168.20.96' -RemoteAddress '192.168.20.0/24' -InterfaceAlias 'Ethernet 2' -Profile Any -EdgeTraversalPolicy Block | Out-Null
    }
    @{success=$true;address='192.168.20.96';port=18080;remote_subnet='192.168.20.0/24'} | ConvertTo-Json | Set-Content -LiteralPath $statusPath -Encoding utf8
} catch {
    @{success=$false;error=$_.Exception.Message} | ConvertTo-Json | Set-Content -LiteralPath $statusPath -Encoding utf8
    throw
}
