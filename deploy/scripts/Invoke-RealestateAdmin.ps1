param(
  [Parameter(Mandatory)]
  [string]$RemoteBash
)
$ErrorActionPreference = "Stop"
$session = & "C:\Users\danie\Scripts\homelab\New-AgentSshSession.ps1" -Agent cursor -HostName 192.168.1.218 -Admin
try {
  $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($RemoteBash))
  $remote = "echo $encoded | base64 -d | sudo -n bash"
  Invoke-Expression ($session.ssh_command + " `"$remote`"")
  exit $LASTEXITCODE
}
finally {
  Invoke-Expression $session.cleanup_command | Out-Null
}
