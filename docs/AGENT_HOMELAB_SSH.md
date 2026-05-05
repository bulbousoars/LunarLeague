# Homelab SSH for agents (OpenBao SSH CA + broker)

Use this flow when you need **SSH into CA-trusted homelab hosts** from **Windows**. Password SSH is disabled on rolled-out hosts; expect `Permission denied (publickey)` unless you use the **ephemeral certificate** session below.

## Read these first (local paths on danspc)

1. **Infrastructure context** (WSL — adjust drive prefix if needed):

   `\\wsl$\Ubuntu\home\dduggan\.claude\projects\-mnt-c-Users-danie\memory\proxmox-infrastructure.md`

   Skim the whole file when touching infra; for SSH CA specifically, find and read:

   - **OpenBao SSH CA update (2026-05-05)**
   - **OpenBao agent broker update (2026-05-05)**
   - **Verified host rollout notes** for CA-trusted nodes/VMs (which IPs trust the CA, principals `cursor-agent`, etc.)

2. **Broker session script**:

   `C:\Users\danie\Scripts\homelab\New-AgentSshSession.ps1`

## Supporting references

- `C:\Users\danie\Scripts\homelab\New-OpenBaoAgentWrappedSecretId.ps1` — human-admin/root `BAO_TOKEN` mints a **one-use wrapped SecretID** for an agent broker AppRole.
- `C:\Users\danie\Scripts\homelab\openbao_agent_broker_roles.json` — broker **role names** and **role IDs** (SecretIDs are never stored here). Keep this file local; do not paste broker material into tickets or public repos.

## Quick usage (wrapped broker → SSH)

Human runs `New-OpenBaoAgentWrappedSecretId.ps1 -Agent cursor` (or approves automation that does), then the agent sets:

```powershell
$env:BAO_WRAPPING_TOKEN = "<wrapped token>"
$env:BAO_ROLE_ID = "<role id>"   # from the wrapped SecretID output / broker roles JSON
powershell -NoProfile -ExecutionPolicy Bypass -File "C:\Users\danie\Scripts\homelab\New-AgentSshSession.ps1" `
  -Agent cursor -HostName 192.168.1.111
```

The script prints an **`ssh_command`** using a **short-lived user certificate** and a temp key dir. Run that command for the session. Use the printed **`cleanup_command`** when finished (removes ephemeral keys).

`-Agent` values: `codex`, `claude`, `gemini`, `cursor`, or `human` (human uses principal `dduggan` / longer TTL per script).

## Alternate: existing OpenBao token

If `BAO_TOKEN` is already set to a token that can sign via `ssh/sign/<role>-agent-15m`, `New-AgentSshSession.ps1` can use it instead of `BAO_WRAPPING_TOKEN` + `BAO_ROLE_ID`.

## Operational notes

- OpenBao listens at **`http://192.168.1.251:8200`** by default in these scripts (override `-OpenBaoAddr` if needed).
- Wrapping tokens are **single-use**; reuse fails by design.
- Example internal target: **`docker-infra` at `192.168.1.111`**; **Lunar League** on VM 218 is documented in the proxmox memory file (`192.168.1.218`, compose under `/mnt/storage/docker/lunarleague/app`).
- If certificates are rejected as **not yet valid**, check **host clock skew** (see proxmox notes for `mediaprod` NTP).
