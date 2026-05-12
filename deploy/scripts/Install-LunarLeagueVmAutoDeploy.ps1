#Requires -Version 5.1
<#
.SYNOPSIS
  Pushes Lunar League VM auto-deploy files to realestate (192.168.1.218) and enables systemd timer.
  Uses OpenBao cursor-admin SSH (same as Invoke-RealestateAdmin.ps1). No secrets in repo.

.NOTES
  Run from Cursor on danspc while homelab is reachable:
    powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\scripts\Install-LunarLeagueVmAutoDeploy.ps1
#>
$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

function ReadB64([string]$rel) {
  $p = Join-Path $here $rel
  return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((Get-Content -LiteralPath $p -Raw)))
}

$bVm = ReadB64 "vm-periodic-deploy.sh"
$bPd = ReadB64 "prod-deploy.sh"
$bSvc = ReadB64 "..\systemd\lunarleague-auto-deploy.service"
$bTmr = ReadB64 "..\systemd\lunarleague-auto-deploy.timer"
$bEnv = ReadB64 "..\systemd\auto-deploy.env.example"

$bash = @"
set -euo pipefail
REPO=/mnt/storage/docker/lunarleague/app
mkdir -p "`$REPO/deploy/scripts"
echo '$bVm' | base64 -d > "`$REPO/deploy/scripts/vm-periodic-deploy.sh"
sed -i 's/\r//g' "`$REPO/deploy/scripts/vm-periodic-deploy.sh"
chmod 755 "`$REPO/deploy/scripts/vm-periodic-deploy.sh"
echo '$bPd' | base64 -d > "`$REPO/deploy/scripts/prod-deploy.sh"
sed -i 's/\r//g' "`$REPO/deploy/scripts/prod-deploy.sh"
chmod 755 "`$REPO/deploy/scripts/prod-deploy.sh"
chown root:lunarleague-deploy "`$REPO/deploy/scripts/vm-periodic-deploy.sh" "`$REPO/deploy/scripts/prod-deploy.sh" || true
mkdir -p /etc/lunarleague
if [ ! -f /etc/lunarleague/auto-deploy.env ]; then
  echo '$bEnv' | base64 -d > /etc/lunarleague/auto-deploy.env
  chmod 640 /etc/lunarleague/auto-deploy.env
  chown root:root /etc/lunarleague/auto-deploy.env
fi
sed -i 's/\r//g' /etc/lunarleague/auto-deploy.env
echo '$bSvc' | base64 -d > /etc/systemd/system/lunarleague-auto-deploy.service
echo '$bTmr' | base64 -d > /etc/systemd/system/lunarleague-auto-deploy.timer
sed -i 's/\r//g' /etc/systemd/system/lunarleague-auto-deploy.service /etc/systemd/system/lunarleague-auto-deploy.timer
chmod 644 /etc/systemd/system/lunarleague-auto-deploy.service /etc/systemd/system/lunarleague-auto-deploy.timer
systemctl daemon-reload
systemctl enable lunarleague-auto-deploy.timer
systemctl restart lunarleague-auto-deploy.timer
systemctl start lunarleague-auto-deploy.service --no-block
sleep 3
systemctl status lunarleague-auto-deploy.service --no-pager -l || true
systemctl list-timers lunarleague-auto-deploy.timer --no-pager || true
"@

$bash = $bash -replace "`r`n", "`n"

$invoke = Join-Path $here "Invoke-RealestateAdmin.ps1"
& $invoke -RemoteBash $bash
Write-Host "Install finished."
