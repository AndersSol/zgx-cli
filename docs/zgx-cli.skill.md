---
name: zgx-cli
description: Use the `zgx` CLI to discover, connect to, configure, and install the curated AI stack on HP ZGX nano / NVIDIA DGX Spark (GB10) devices over SSH. Covers exact command syntax, flags, the hard-won gotchas (default user is `hp` but NVIDIA-imaged devices use a custom user; mDNS discovery is flaky; `install`/`verify`/`uninstall` require `--host` while `connect`/`health` take a positional host), and which features are hardware-verified. Use when the user wants to run zgx commands against a device, set up passwordless SSH access, install/verify/uninstall apps, check device health, register mDNS, pair two devices, or troubleshoot zgx discovery/connection failures.
---

# zgx CLI — operating guide

`zgx` is a portable Go CLI (module `github.com/AndersSol/zgx-cli`, binary `zgx`) that
configures **HP ZGX nano** = **NVIDIA DGX Spark (GB10)** devices over SSH. It is a
**client that runs on the Mac/laptop** and drives the device remotely.

## Mental model (read first)

- **`zgx` runs on the controlling machine (Mac), NEVER on the device.** If you see a
  prompt like `<user>@spark-XXXX:~$`, you are *on the device* — `zgx` will report
  "command not found". Type `exit` to return to the Mac, then run `zgx`.
- The device is reached by **hostname** (`spark-XXXX` / `spark-XXXX.local`), **IPv4**
  (`<device-ip>`), or **IPv6 ULA** (`fd3c:…`). All three work as a host argument.
- `connect` installs an ed25519 key into the device's `authorized_keys`. After that,
  `health`, `install`, etc. authenticate with that key — no password for SSH itself.
  (`install`/`uninstall` still need the device's **sudo** password for `apt`.)

## ⚠️ Critical gotchas (these cost hours — apply them every time)

1. **Default `--user` is `hp` everywhere, but that is usually WRONG.** The `hp` user
   only exists on HP's own factory image. A device **re-imaged with NVIDIA's DGX OS
   recovery image** uses **the username created during first-boot setup** (whatever you chose).
   → **Always pass `--user <actual-username>`.** Symptom of the wrong user:
   `ssh: handshake failed: unable to authenticate, attempted methods [none password],
   no supported methods remain` — that is a wrong **username** (or wrong password), not
   a network problem; the connection reached the SSH auth stage fine.
2. **mDNS discovery is flaky.** `zgx discover` sends one query with no retransmit, so it
   often prints `No ZGX devices found` on a cold cache. **Retry 2–4×**, or skip discovery
   and pass the host directly (`spark-XXXX.local`, the IPv4, or the IPv6 ULA).
3. **`install` / `verify` / `uninstall` require `--host` (no auto-discovery).**
   `connect` / `health` / `dns-register` / `pair*` take the host as a **positional
   argument** instead. Don't mix them up.
4. **Discovery often surfaces only IPv6 ULA addresses** (no IPv4 A-record). The device
   *does* have IPv4 — read it from the SSH login banner (`IPv4 address for enP7s7: …`)
   and use it directly with `--host` if IPv6 is inconvenient.
5. **First `connect` shows a TOFU host-key prompt** (`ED25519 fingerprint: … Do you
   trust it? Type yes:`). You must type the full word `yes`. Typing `no` aborts with
   `unknown SSH host … rejected` — that is expected, not a bug.
6. **Hostname changed across re-image:** HP factory image → `zgx-XXXX`; NVIDIA DGX OS
   recovery image → `spark-XXXX`. Same device, different advertised name.
7. **Agent sandboxes may block local mDNS.** If `zgx discover` fails with
   `Failed setting up UDP server ... 224.0.0.251:5353: bind: operation not permitted`,
   that is a local sandbox/network-permission problem, not a device problem. Re-run the
   same read-only discovery with local-network permission/escalation instead of changing
   ZGX settings.
8. **Don't put temporary `known_hosts` directly in `/tmp` or `/private/tmp`.** `zgx`
   secures the parent directory of the `--known-hosts` path and may fail with `chmod
   /private/tmp: operation not permitted`. Use the normal `~/.ssh/known_hosts`, or a
   tool-owned directory such as `<workspace>/work/zgx-known-hosts/known_hosts`.

## Command reference

Global: `-h/--help`, `-v/--version`. Run `zgx <cmd> --help` for any subcommand.

### Discovery & connection
```bash
zgx discover [--timeout 5]                      # mDNS browse; retry if empty (gotcha #2)
zgx connect [<host>] [--user U] [--port 22] \
            [--alias NAME] [--password P] \
            [--known-hosts FILE] [--discover-timeout 6]
```
- `connect` with **no host** auto-discovers: 0 found → error, 1 → auto-select,
  2+ → numbered picker. With a host arg, connects directly.
- Prompts for the device password (hidden) unless `--password` given, generates/reuses
  `~/.ssh/id_ed25519`, appends the pubkey to the device, writes a `~/.ssh/config` alias
  (defaults to the hostname), then tests key-based access.
- After success: `ssh <alias>` (e.g. `ssh spark-XXXX`) logs in passwordless.

### Health
```bash
zgx health <host> [--user U] [--port 22] [--identity KEY] [--known-hosts FILE]
```
Runs a key-auth SSH test. Prints `healthy` on success. Run `connect` first (needs the key).

### Apps (install / verify / uninstall)
```bash
zgx list                                         # local catalog, no SSH
zgx install <app...|--all> --host H [--user U] [--yes] [--identity KEY]
zgx verify  <app...|--all> --host H [--user U]
zgx uninstall <app...|--all> --host H [--user U]
```
- `--host` is **required**. `--user` defaults to `hp` (gotcha #1).
- `install`/`uninstall` print a **raw command plan** (⚠ marks any `curl|sh`), ask for
  **confirmation** (skip with `--yes`), then prompt **`Sudo password:`** (the device
  user's password, fed to `sudo -S` for `apt`).
- `install` expands dependencies in topological order; `uninstall` does **not** expand
  (removes only what you name). Safe smoke test: `zgx install btop --host spark-XXXX --user <user>`.
- **Reading the report:** the final lines are `Installed:`, `Already installed:`, `FAILED:`.
  A lone **`FAILED: -`** means the dash = **empty list = nothing failed** (success), NOT
  "an app called -". The command exits non-zero only when `Failed` is non-empty.
  On fresh DGX OS, `base-system`'s apt deps are usually pre-present → `Already installed`.

### mDNS registration
```bash
zgx dns-register <host> [--user U] [--port 22] [--identity KEY] [--known-hosts FILE]
```
Writes an avahi service on the device for stable discovery (restart of avahi is non-fatal).

### Pairing (needs TWO devices)
```bash
zgx pair <host> [...]        zgx unpair <host> [...]        zgx pair-details <host> [...]
```
ConnectX NIC pairing / netplan. Cannot be exercised with a single device.

### Saved devices
```bash
zgx config add <alias> <host> [--user U] [--port 22] [--identity KEY]
zgx config list
zgx config remove <alias>
```
XDG-persisted device list. (Note: as of this writing the host commands do not yet auto-
read the saved `--user`; still pass `--user` explicitly.)

## Operator recipes (verified, self-checking)

These move the skill from reference to operator: each step says whether **you (the agent)**
run it via Bash and self-check, or **hand it to the user** because it reads a hidden
password from the TTY (no flag can supply it safely). Assert success only on the EXACT
signatures below. Verified against a real DGX Spark (DGX OS 7.5.0); design-reviewed
by Codex (7 false-success / hang / secret-leak holes closed).

### Who runs what (secret-prompt split)
- **Agent runs non-interactively** (no secret prompt): `discover`, `health`, `verify`,
  `list`, `pair-details`, `config list`.
- **Hand to the user** (hidden password prompt — never pipe it, it hangs on `term.ReadPassword`):
  - `connect` → `Password for <user>@<host>:` (device login password).
  - `install` / `uninstall` / `dns-register` / `pair` / `unpair` → `Sudo password…` (device sudo).
  After the user runs one, the agent runs the matching read-only step to self-check.

### Success signatures (exact, from the code)
| Step | SUCCESS when stdout contains | Failure signal |
|---|---|---|
| connect | `Key-based access works.` | hint `the username or password was not accepted` |
| health | `<host>: healthy` | `<host>: unreachable:` |
| install | `FAILED: -` (dash = empty list) | `FAILED:` followed by any id |
| verify | exact line `✓ <app> installed` | `✗ <app> missing` |
| dns-register | `Service file written: true` | `Service file written: false` |

### Recipe A — Bring a device online (cold → passwordless SSH)
0. **Agent** — establish local context first:
   ```bash
   command -v zgx && zgx --version
   ```
   If the user already gave a username, save it as `<U>` and use it immediately. If they
   did not, ask for the username created during first-boot setup before interpreting auth
   failures; do not keep retrying with the default `hp`.
1. **Agent** — discover with retry. `zgx discover` **exits 0 even when it prints
   `No ZGX devices found`**, so gate on a positive device row, not the exit code. If a
   sandbox blocks UDP 5353, re-run with local-network permission/escalation:
   ```bash
   found=
   for i in 1 2 3 4; do
     out=$(zgx discover); printf '%s\n' "$out"
     printf '%s\n' "$out" | grep -Eq '  .*:22.*  \(.*\)$' && { found=1; break; }
     sleep 2
   done
   test -n "$found" || echo "no device after 4 tries — ask the user for the host/IP"
   ```
   Take the `spark-*` / `zgx-*` hostname. If `found` is empty, ask the user for the host/IP
   (e.g. from the device's SSH login banner).
2. **Agent** — confirm basic reachability before trying SSH auth:
   ```bash
   nc -vz -w 5 <host>.local 22 || nc -vz -w 5 <ip-or-ipv6> 22
   ```
   A successful TCP check proves the device is reachable even if key-auth is not yet set up.
3. **Agent** — if `<U>` is known and key access may already exist, run:
   ```bash
   zgx health <host>.local --user <U>
   ```
   Success is `<host>.local: healthy`. `attempted methods [none publickey]` means SSH was
   reached but key-based access is not set up for that user. `hp` failing does not matter
   when the real user is different.
4. **User** — if health is not already healthy, run `zgx connect <host> --user <U>`
   (pass the host from step 1; bare
   `zgx connect` re-runs discovery and may pick a different device). They type the device
   password, then `yes` at the TOFU fingerprint prompt. Confirm their output contains
   `Key-based access works.`
5. **Agent** — gate it: `zgx health <host> --user <U>` → require `<host>: healthy`.
   `unreachable` + the username/password hint ⇒ wrong `--user` or password; stop.

### Recipe B — Install an app and confirm it (idempotent)
1. **User** — `zgx install <app> --host <host> --user <U>` (confirm `yes`, then sudo
   password). Provisional success = `FAILED: -` AND `<app>` as a whole comma-separated id
   under `Installed:` or `Already installed:`.
2. **Agent** — authoritative confirm with an EXACT whole-line match (don't let `top` match
   `btop`):
   ```bash
   zgx verify btop --host spark-XXXX --user <user> | grep -qx "✓ btop installed" \
     && echo OK || echo "install did not take"
   ```
   `✗ <app> missing` ⇒ install did not take.

### Recipe C — Register for stable discovery
1. **User** — `zgx dns-register <host> --user <U>` (sudo password). Success =
   `Service file written: true`. `Avahi restarted: false` is non-fatal — the service file
   is written and activates on the next reboot.

### Operator guardrails
- Always pass `--user <name>` (default `hp` mismatches NVIDIA-imaged devices).
- **Never** run `uninstall`, `unpair`, `pair`, or any `--all` command unless the user
  explicitly asks — `pair` writes/applies netplan, `unpair` removes ConnectX config; both
  are material network changes.
- **Never** use `zgx connect --password …` — it puts the device login password in argv and
  shell history. Let the user type it at the prompt.
- Prefer a pinned host/IP over discovery for repeat runs (mDNS misses on a cold cache).
- Don't feed passwords to the secret-prompt commands; hand those to the user.

## Hardware-verification status (against a real DGX Spark, DGX OS 7.5.0)

| Command | Status |
|---|---|
| `discover` | ✅ verified (flaky — see gotcha #2) |
| `connect` | ✅ verified — key bootstrap, TOFU prompt, ssh-config, key-auth test all work |
| `health` | ✅ verified — returns `healthy` |
| `list` | ✅ verified (local) |
| `install` | ✅ verified — `btop` installed, dependency expansion (base-system→btop), `sudo -S` over SSH, `FAILED: -` (none) |
| `verify` | ✅ verified — `zgx verify btop` → `✓ btop installed` (key-auth, no sudo prompt) |
| `uninstall` | ⏳ not yet confirmed on hardware (engine shares install's verified SSH/sudo path) |
| `dns-register` | ✅ verified — device ID generated, service file written, avahi restarted |
| `pair` / `unpair` / `pair-details` | ⛔ needs two devices |

## Known issues / backlog (don't re-diagnose — these are understood)

- **mDNS retransmit:** brutella/dnssd sends the PTR query once → intermittent misses.
  Workaround: retry `discover`, or pass the host directly.
- **IPv4 A-record resolution:** browse entries frequently carry only IPv6 ULA; IPv4 is
  reachable but not always advertised. Use `--host <ipv4>` when needed.
- **`--user hp` default:** mismatches NVIDIA-imaged devices. Until made configurable per
  saved device, always pass `--user`.
