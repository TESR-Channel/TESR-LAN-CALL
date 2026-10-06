# TESR LAN Call - open Windows Firewall so phones/iPads on the LAN can connect.
# Runs elevated (installer, or "fix connection" button in the app).
param([string]$Exe, [string]$Tcp = "47800,47843", [string]$Udp = "47801")
$ErrorActionPreference = 'SilentlyContinue'
$name = 'TESR LAN Call'

if (Get-Command New-NetFirewallRule -ErrorAction SilentlyContinue) {
  # Remove BLOCK rules Windows creates when someone presses Cancel on the firewall popup
  Get-NetFirewallApplicationFilter | Where-Object { $_.Program -like '*TESR-LAN-Call*' } |
    Get-NetFirewallRule | Where-Object { $_.Action -eq 'Block' } | Remove-NetFirewallRule
  Get-NetFirewallRule -DisplayName "$name*" | Remove-NetFirewallRule
  if ($Exe) {
    New-NetFirewallRule -DisplayName $name -Direction Inbound -Action Allow -Program $Exe -Profile Any | Out-Null
  }
  New-NetFirewallRule -DisplayName "$name (TCP)" -Direction Inbound -Action Allow -Protocol TCP -LocalPort ($Tcp -split ',') -Profile Any | Out-Null
  New-NetFirewallRule -DisplayName "$name (UDP)" -Direction Inbound -Action Allow -Protocol UDP -LocalPort ($Udp -split ',') -Profile Any | Out-Null
  if (Get-NetFirewallRule -DisplayName "$name (TCP)") { exit 0 } else { exit 2 }
} else {
  netsh advfirewall firewall delete rule name="$name" | Out-Null
  netsh advfirewall firewall delete rule name="$name (TCP)" | Out-Null
  netsh advfirewall firewall delete rule name="$name (UDP)" | Out-Null
  if ($Exe) { netsh advfirewall firewall add rule name="$name" dir=in action=allow program="$Exe" enable=yes profile=any | Out-Null }
  netsh advfirewall firewall add rule name="$name (TCP)" dir=in action=allow protocol=TCP localport=$Tcp enable=yes profile=any | Out-Null
  netsh advfirewall firewall add rule name="$name (UDP)" dir=in action=allow protocol=UDP localport=$Udp enable=yes profile=any | Out-Null
  exit $LASTEXITCODE
}
