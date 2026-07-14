# Kanata hot-reload mechanism — primary-source research

**Method:** primary sources only — GitHub source tree (`jtroo/kanata`) + clap-derived
`--help` doc strings + `docs/config.adoc` + `README.md` + GitHub release/CHANGELOG
notes via `gh`. No secondary write-ups cited.

## Repo + version

| Field | Value |
|---|---|
| Upstream repo | `https://github.com/jtroo/kanata` |
| Owner org | `jtroo` (Jan Tache, original author; `kanata-keyberon` was the historical crate name — the live binary crate is the `kanata` workspace at this repo) |
| Latest tagged release at research time | **v1.12.0**, published `2026-07-05T06:39:27Z`, commit `02242ef761ef43b860eac30067ce86904be7d466` (`ver: 1.12.0`, 2026-07-04) |
| Latest commit on `main` at research time | `c7978d4af0a7141a0dd0e5928f388ff94b10576f` (2026-07-08) |
| Source I read | shallow clone at `/tmp/opencode/kanata-research/kanata-1.12.0` (tag `v1.12.0`), plus `kanata-main` (full history, fetched and unshallowed) for change archaeology |
| Cargo package version (`src/Cargo.toml`) | `version = "1.12.0"` |

Confirmation that this is the right repo:
- Repo `default_branch = main`, `stargazers_count = 7610` (via `gh api`).
- `README.md:24` links to `./docs/config.adoc` from this repo.
- `README.md:301` cites the underlying `keyberon` crate that gives kanata its real name origin.
- Cargo `[package] homepage = repository = https://github.com/jtroo/kanata`
  (`Cargo.toml:12-13`).

## CLI flags that take a config

From `src/main_lib/args.rs:16-131` (the `Args` struct parsed by clap). The full
set of flags that affect config / reload behaviour:

| Flag | Type | Purpose |
|---|---|---|
| `-c`, `--cfg PATH` (`Option<Vec<PathBuf>>`) | repeatable | One or more config files. **First entry is the file loaded on startup; later entries are cycle targets for `lrld-next/lrld-prev/lrld-num` and TCP `ReloadNext/ReloadPrev/ReloadNum`.** (`args.rs:36-37`, `config.adoc:5374-5384`, `config.adoc:836-843`) |
| `--cfg-stdin` (`bool`) | one-shot | Read config from stdin instead of a file. Bypasses the `--cfg` path entirely. (`args.rs:40-41`, `main.rs:93-106`) |
| `--check` (`bool`) | one-shot | **Validate config and exit without starting the keymap loop.** Equivalent of `wasm::check_config` (`args.rs:96-97`, `main.rs:121-142`). The only CLI command that accepts a config without running kanata. |
| `-p`, `--port PORT-or-IP:PORT` (`Option<SocketAddrWrapper>`) | one-shot, feature `tcp_server` | Enable the TCP server. **Required for the live-reload mechanism to be reachable from outside the process.** (`args.rs:46-54`, `config.adoc:5386-5414`) |
| `-l`, `--list` | per-platform | List grabbable keyboards and exit. |
| `--symlink-path PATH` (Linux/Android only) | one-shot | Path for a symlink to the new kanata virtual evdev device. |
| `-w`, `--wait-device-ms N` (Linux/Android only) | one-shot | A startup-only tuning flag. **Not a config flag.** |
| `--nodelay` (`bool`) | one-shot | Skip the 2 s startup idle window. |
| `--no-wait` (`bool`) | one-shot | Skip "Press enter to exit" prompt (for service managers). |
| `--emergency-exit-code N` (`i32`, default `0`) | one-shot | Custom exit code for `LCtrl+Space+Escape` emergency exit. |
| `-q`, `--quiet`, `-d`, `--debug`, `--trace`, `--log-layer-changes` | one-shot | Logging knobs. |
| `--release-grab-on-lock` (macOS only) | one-shot | macOS grab-release policy. |
| `--macos-request-permissions` (macOS only) | one-shot | Request Accessibility permission and exit. |

**No `--reload`, `--reload-file`, `--watch`, `--hup` flag exists in v1.12.0.**

### Default search paths (no `-c` passed)

`src/lib.rs:30-46` `default_cfg()` — when `-c` is omitted:

1. `./kanata.kbd` (current working directory), if it is a regular file.
2. `<config_dir>/kanata/kanata.kbd`, where `config_dir = dirs::config_dir()`.

Per the `--help` string itself (`args.rs:18-35`):

- Windows: `C:\Users\user\AppData\Roaming\kanata\kanata.kbd`
- macOS:   `$HOME/Library/Application Support/kanata/kanata.kbd`
- Linux/Android: `$XDG_CONFIG_HOME/kanata/kanata.kbd`

**Reality check:** a runtime-bail happens if no config file exists
(`main.rs:108-119`: `bail!("Could not find the config file ({}) ...")` and
`bail!("No config files provided ...")`). `-c` is the only stable way to point
kanata at non-default paths.

## Reload mechanism

### Headline

Live reload is **TCP-driven from outside the process**, not signal-driven. There
is **no SIGHUP / SIGUSR1 / SIGUSR2 / inotify-watcher hook** anywhere in v1.12.0.
The `signal-hook` crate is in `Cargo.toml:32` only as a low-level dependency
for SIGCHLD/SIGTERM plumbing in `src/oskbd/linux.rs:7-12, 758-762` and one
`signal_hook::consts::SIGTERM` raise in `src/kanata/mod.rs:2708` (kanata
self-raising SIGTERM from the emergency-exit path). It is **never wired to a
config-reload action**.

Cross-checked: `grep -rn 'SIGHUP\|SIGUSR\|signal.hup\|sig_hook\|HUP' src/`
returns zero matches (only `SIGTERM` / `SIGCHLD` plumbing, unrelated). README
"Notable features" line simply says *"Live reloading of the configuration for
easy testing of your changes"* (`README.md:188`) — no spec for *how*.

### Three ways a reload is triggered in v1.12.0

1. **In-config key action** — `lrld`, `lrld-next` (`lrnx`), `lrld-prev` (`lrpv`),
   `(lrld-num N)` (1-based), `(lrld-file PATH)` (parser added later — see
   `parser/src/cfg/live_reload.rs:6-40`). All five assemble into
   `CustomAction::LiveReload{File,Num}` consumed in
   `src/kanata/mod.rs:1856-1892` which calls `request_live_reload*` methods
   (`mod.rs:2009, 2049, 2063, 2077, 2096`).
2. **TCP server command** — JSON-over-TCP sent to a connected client.
   Iterator through `ClientMessage::{Reload, ReloadNext, ReloadPrev,
   ReloadNum, ReloadFile}` (`tcp_protocol/src/lib.rs:88-144`) → dispatched from
   `src/tcp_server.rs:56-106`'s `handle_reload_with_wait` →
   `Kanata::handle_client_command` (`mod.rs:2020-2046`) → same
   `request_live_reload*` setters.
3. **Windows GUI tray reload menu** (`src/gui/win.rs:844-947`,
   `src/kanata/windows/mod.rs:138-165`) — calls the same `request_live_reload*`
   helpers via the embedded `kanata_state_machine` library API. Same flow.

There is **exactly one** setter that arms a reload: `live_reload_requested =
true` (`mod.rs:2010, 2050, 2064, 2085, 2101`). Each setter also selects a new
`cur_cfg_idx` index into the cached `cfg_paths: Vec<PathBuf>` (built at startup
from `-c` flags).

### How the reload actually executes

The event loop polls in two places:
`src/kanata/mod.rs:951-968` (`handle_time_ticks`) and `mod.rs:2301-2310` and
`2370-2379` (Windows event loops). The gate is identical in all three:

```rust
if self.live_reload_requested
   && ((self.prev_keys.is_empty() && self.cur_keys.is_empty())
       || self.ticks_since_idle > 1000) {
    self.live_reload_requested = false;
    if let Err(e) = self.do_live_reload(tx) {
        log::error!("live reload failed {e}");
    }
}
```

So a queued reload executes only when **no keys are currently pressed** OR a
1-second idle fallback has elapsed (e.g. Win+L → lock screen with stuck keys —
comment at `mod.rs:955-963`). Reload then calls `do_live_reload` (`mod.rs:732-869`)
which calls `cfg::new_from_file(&self.cfg_paths[cur_cfg_idx])` then mutates
`self.layout`, `self.layer_info`, `self.overrides`, `self.sequences`,
`self.virtual_keys`, etc. (`mod.rs:733-793`).

### History that's relevant

- `lrld` was introduced in **commit `3bb1ff7` 2022-04-25** *"Add live reload"*
  by Jan Tache (`git log -S 'do_live_reload' --all | tail`). `lrld-num`
  appeared on 2023 (`git log` shows PR #701, commit `ae855e7`).
- A `--watch` flag **was** added on 2025-07-26 by PR #1707 (commit `4f0a1c6`)
  using inotify-style file watching + a 500 ms debounce, then **removed on
  2025-08-20** by PR #1747 (commit `1fcac5b` *"fix: remove --watch feature due
  to reliability issues"*) — the commit message states explicitly:

  > *The core issue stems from architectural challenges in the event loop …*
  > ***The TCP-based reload mechanism is already available and highly reliable:**
  > `kanata --cfg config.kbd --port 12345` … `echo '{"Reload": {}}' | nc localhost 12345`*

- **PR #1714 (commit `bf57c88`, 2025-07-28)** added the JSON TCP reload
  commands. First shipping release that contains them: **v1.10.0**
  (`git describe --contains bf57c88` → `v1.10.0-prerelease-1~34`; v1.10.0
  changelog list explicitly says *"added: TCP commands for live reload of
  configuration"*).
- **PR #1882 (commit `63069a5`, 2025-12-21)** added the `wait` / `timeout_ms`
  optional fields in `ClientMessage::{Reload, ReloadNext, ...}` so callers can
  block on actual completion and receive a `ReloadResult` message.
  First containing release: **v1.11.0**.
- `HellO { }` capability handshake was also added in v1.11.0
  (`tcp_protocol/src/lib.rs:38-43`, `141-143`).

### Wire format (what kanta should send)

Plaintext JSON, **newline-terminated**, one message per line
(`docs/config.adoc:5396-5409`).

Reload current (cheapest, equivalent to `lrld`):

```json
{"Reload":{}}\n
```

Reload a specific path (equivalent to `lrld-file`):

```json
{"ReloadFile":{"path":"/absolute/path/to/config.kbd"}}\n
```

Reload a specific arg-list slot (equivalent to `(lrld-num N)`, **0-based in TCP**,
1-based in the config side):

```json
{"ReloadNum":{"index":1}}\n
```

Reload next/prev (cycle `-c` list, equivalent to `lrnx`/`lrpv`):

```json
{"ReloadNext":{}}\n
{"ReloadPrev":{}}\n
```

Server response is always either:

```json
{"status":"Ok"}\n
{"status":"Error":{"msg":"config index 5 out of bounds: only 2 configs available (0-1)."}}\n
```

(`tcp_protocol/src/lib.rs:63-76` — verified against the example client output
at `example_tcp_client/src/main.rs:108-112`.)

`config.adoc:5509-5516` documents the sync confirm variant introduced in v1.11.0:

```json
{"Reload":{"wait":true,"timeout_ms":5000}}\n
```

Server replies with `{"status":"Ok"}\n` immediately on accepting the request,
then a follow-up `{"ReloadResult":{"ok":<bool>,"timeout_ms":<u64>|null}}\n`
after the reload completes (or times out at `timeout_ms`, default 5 s). Wire
implementation: `src/tcp_server.rs:57-106` `handle_reload_with_wait`.

The `ServerMessage::ConfigFileReload {"new": "<path>"}` event is broadcast to
all subscribers on every successful reload (`mod.rs:801-816`) — useful for
clients that want to react.

## Reload caveats

| Caveat | Evidence |
|---|---|
| **TCP server must be enabled at kanata startup** with `-p`/`--port`. There is no way to enable it later or signal it to start. | `args.rs:46-54`, `main.rs:203-219`. |
| **No SIGHUP / signal hook** for reload. Killing-and-restarting kanata loses all sticky/sequence/tap-hold state and reopens the virtual device; not equivalent to live reload. | grep `SIGHUP\|SIGUSR` → 0 hits for reload. `signal-hook` crate used only for SIGTERM/SIGCHLD plumbing unrelated to config. |
| **Reload only fires when keyboard is idle** (`prev_keys.is_empty() && cur_keys.is_empty()`) OR after a 1 s stuck-key fallback. A reload request still queued while you type will block until you stop pressing. | `mod.rs:951-968`, `mod.rs:2302-2310`, `mod.rs:2371-2379`. |
| **Live reload does *not* apply device-related config** — e.g. `linux-dev`, `linux-use-trackpoint-property`, `macos-dev-names-include`, `windows-only-windows-interception-keyboard-hwids`, the Interception `mouse-movement-key` opt-in. | `docs/config.adoc:796-799`, `docs/config.adoc:4708-4711` (Windows-interception `mouse-movement-key` line: *"this option must be present on startup to enable mouse movement event collection, so restart is required to enable it"*). |
| **Active layer resets to the first `deflayer`** after every successful reload (comments confirm this is intentional, not a bug). Held modifiers on the active layer are dropped (`mod.rs:848` clears `PRESSED_KEYS`; `tick_states` resumes from the new origin). | `docs/config.adoc:806-807` + `mod.rs:818-821, 848`. |
| **No reload-watch on disk** — the `--watch` flag (file system watcher) was added in PR #1707 / v1.9-era and **removed** in PR #1747 due to reliability issues with the event loop. The TCP workflow is the project-recommended replacement per the removal commit message. | `git log -S 'do_live_reload' / -S 'request_live_reload'` shows `4f0a1c6` then `1fcac5b`; `1fcac5b` commit message confirms. |
| **Empty-event-loop bug workaround**: on Windows LLHOOK with stuck keys (Win+L → lock screen releases unreceived), the reload will force-fire after 1 s idle despite keys appearing held. | `mod.rs:955-963`. |
| **`(lrld-file PATH)` exists in the config-side parser** but `LiveReloadFile` was added later — the public enum-mapping for it lives in `parser/src/cfg/live_reload.rs:20-40`. The same path appears as TCP `ReloadFile`. | `live_reload.rs:20-40`, `tcp_protocol/src/lib.rs:126-139`, `mod.rs:2096-2109`. |
| **Reload only knows paths registered in `cfg_paths`**, except `ReloadFile` which appends a runtime-discovered path (`mod.rs:2102`: `self.cfg_paths.push(new_path)`). On next `ReloadNext`, the just-injected path stays reachable. | `mod.rs:2096-2109`. |

Permissions: the TCP server listens on the address passed to `-p`; the docs
say "on all IP addresses" for a bare port, and `IP:PORT` for a specific bind
(`docs/config.adoc:5390-5395`). Kanata does **not** require a TTY for reload
itself — reload works in headless contexts as long as the OS permits granting
the Accessibility (macOS) / input group (Linux) / driver install (Windows)
state that the initial grab needed. README:24 `kanata --cfg <file>` works.

## Conflict / invalid config behaviour

Direct reading of `src/kanata/mod.rs:732-743`:

```rust
fn do_live_reload(&mut self, _tx: &Option<Sender<ServerMessage>>) -> Result<()> {
    let cfg = match cfg::new_from_file(&self.cfg_paths[self.cur_cfg_idx]) {
        Ok(c) => c,
        Err(e) => {
            log::error!("{e:?}");
            #[cfg(feature = "tcp_server")]
            {
                self.last_reload_ok = false;
            }
            bail!("failed to parse config file");
        }
    };
```

Sequence on a parse-error reload:

1. `cfg::new_from_file` returns `Err(miette::Report)`.
2. Full miette diagnostic is logged at `error` level via `log::error!("{e:?}")`.
3. With TCP server feature: `last_reload_ok = false` is recorded so the (optional)
   `ReloadResult` follow-up message will have `ok: false`.
4. `do_live_reload` returns `Err`.
5. The caller wraps it: `log::error!("live reload failed {e}")`
   (`mod.rs:966-967`, `2308`, `2377`).
6. **No mutation to `self.layout` happens — the previously-loaded config keeps
   running unchanged.** This matches `docs/config.adoc:803-805`:
   *"You can put the `lrld` action onto a key to live reload your configuration
   file. If kanata can't parse the file, the previous configuration will
   continue to be used."*
7. For the no-keyboard-bound trigger (TCP), the immediate JSON response is
   `{"status":"Ok"}` (because the request was queued, not because parsing
   succeeded). kanta must use `{"Reload":{"wait":true,"timeout_ms":N}}` (v1.11+
   only) to receive `ReloadResult{ok:false,...}` after the parse attempt.
   Without `wait`, the Err is reported only in kanata's own log output, not to
   the TCP caller.

There is **no automatic exit** on bad config. The process keeps running with
the old config; only `panic!` or external `kill` exits kanata.

## Recommended kanta integration

### Preconditions

kanta must launch kanata with the TCP server enabled — the only public
reload channel in v1.12.0.

```
kanata --cfg <kanta-managed-config-path> --port <chosen-port>
```

`<kanta-managed-config-path>` is whatever file kanta writes the rendered config
to. `<chosen-port>` is whatever port kanta controls (likely a fixed localhost
loopback port chosen by kanta itself).

### Recommended call shape (apply to kanata ≥ v1.11.0)

After kanta writes/rewrites the active config file:

1. **Validate first** with `kanata --check --cfg <path>` as a cheap dry-run.
   This uses the parser only — `args.rs:96`, `main.rs:121-142` — and exits 0 on
   success / non-zero on parse error. Caveat: `--check` does not run the
   engine, so it does **not** catch action-time evaluation errors that
   `do_live_reload` would surface at runtime. It catches syntax + most static
   validation. (`wasm::check_config` in the wasm target is the equivalent
   for browser-side validation.)
2. **Then push to running kanata** via a single JSON line on the TCP socket:

   ```json
   {"Reload":{"wait":true,"timeout_ms":5000}}\n
   ```

   Send, then read until either `{"ReloadResult":{"ok":true,...}}` or
   `{"ReloadResult":{"ok":false,...}}` arrives, or until the timeout fires.
3. On `ok:false`, surface the kanata log output (kanta would need to attach
   to kanata's stderr to render the miette diagnostic) and **keep the
   previously-loaded config live**. No need to restart kanata.
4. On `ok:true`, optionally also send `{"ConfigFileReload"-event-driven:
   "no-op"}` — actually, no — it's a server-pushed message, so just trust
   the existing kanata layer-change broadcast.

### What to do on kanata ≥ v1.10.0 but < v1.11.0

If kanta detects kanata < v1.11.0 (e.g. via a TCP `{"Hello":{}}` HelloOk
probe — introduced with the protocol handshake — see
`tcp_protocol/src/lib.rs:38-44, 141-143`): `wait`/`timeout_ms` doesn't exist
yet. Send `{"Reload":{}}\n` and accept a 5-second best-case "did it fire or
did it fail" blind window. The immediate server reply will be
`{"status":"Ok"}` regardless of the parse outcome — that response only
means the request was queued (`tcp_server.rs:66-77, 80-104`). The TCP author
notes this clarification in PR #1714 commit message: *"Before: Fire-and-forget
commands with no client feedback. After: Request/response protocol with
standardized feedback."*

### What to do if kanta must support kanata < v1.10.0

TCP `Reload*` commands are absent. Fall back to the in-config `lrld`
key-action alias — but triggering it externally is not possible without a
key press. Effective options:

- Bind a hot key to `(lrld-file <path>)` and synthesise the key event via the
  Linux/macOS virtual input back-channel (very fragile, OS-specific, and kanta
  would re-introduce exactly the inotify-style architecture that the kanata
  project removed).
- Treat it as **no live reload** and document this in kanta's supported-kanata
  matrix; ask the user to restart kanata manually after a config save.

### What to do if kanta must work without the TCP server

Not realistic on v1.10.0+ — without `-p`, **there is no external reload
channel**. kanta should re-launch kanata with a chosen port. If the user
explicitly disabled the TCP server, kanta can't override; fall back to the
"restart kanata on save" UX.

## Fallback if no live reload exists (force-restart path)

This is the documented manual mechanism — `README.md:58-64`: *"Running kanata
currently does not start it in a background process. You will need to keep the
window that starts kanata running to keep kanata active."* No CLI flag ever
replaces the running process; kanta would have to:

1. `kill <pid>` (SIGTERM) of the running kanata — clean exit, no input-leak
   OS recovery needed because `signal_hook::low_level::emulate_default_handler`
   passes SIGTERM on to the OS in the Linux path (`src/oskbd/linux.rs:758-762`).
2. Re-exec `kanata --cfg <path> --port <port> --nodelay --no-wait`.
3. Re-establish the TCP connection on next reload.

For `--emergency-exit-code`, the `LCtrl+Space+Escape` triple-chord exits with
that code (`args.rs:111-115`, `main.rs:156-159`); useful for service-manager
restarts but not for kanta-driven config save.

## Sources

- `https://github.com/jtroo/kanata` (repo, v1.12.0 = commit `02242ef761ef43b860eac30067ce86904be7d466`)
- `/tmp/opencode/kanata-research/kanata-1.12.0/README.md`
- `/tmp/opencode/kanata-research/kanata-1.12.0/Cargo.toml:1-20`
- `/tmp/opencode/kanata-research/kanata-1.12.0/src/main.rs:23-232, 244-256`
- `/tmp/opencode/kanata-research/kanata-1.12.0/src/main_lib/args.rs:6-131`
- `/tmp/opencode/kanata-research/kanata-1.12.0/src/lib.rs:30-46`
- `/tmp/opencode/kanata-research/kanata-1.12.0/src/kanata/mod.rs:732-869, 951-968, 2009-2109, 2301-2310, 2370-2380`
- `/tmp/opencode/kanata-research/kanata-1.12.0/src/tcp_server.rs:56-106`
- `/tmp/opencode/kanata-research/kanata-1.12.0/tcp_protocol/src/lib.rs:10-161`
- `/tmp/opencode/kanata-research/kanata-1.12.0/example_tcp_client/src/main.rs:50-220`
- `/tmp/opencode/kanata-research/kanata-1.12.0/parser/src/cfg/live_reload.rs:1-40`
- `/tmp/opencode/kanata-research/kanata-1.12.0/parser/src/cfg/defcfg.rs:109-130, 749-767`
- `/tmp/opencode/kanata-research/kanata-1.12.0/parser/src/cfg/mod.rs:82-83, 1670-1671`
- `/tmp/opencode/kanata-research/kanata-1.12.0/parser/src/custom_action.rs:65-75` (LiveReload* action variant list)
- `/tmp/opencode/kanata-research/kanata-1.12.0/parser/src/cfg/tests.rs:1402-1403` (`notify-cfg-reload yes` only affects GUI toast signalling, not reload triggering)
- `/tmp/opencode/kanata-research/kanata-1.12.0/docs/config.adoc:766-845, 5374-5414, 5487-5517, 2464, 4656-4657, 4700-4711, 5382-5384`
- `/tmp/opencode/kanata-research/kanata-1.12.0/docs/kmonad_comparison.md` (no reload mention)
- Git history via `kanata-main` (full, fetched):
  - `3bb1ff7` 2022-04-25 *"Add live reload"* (initial `lrld` action)
  - `ae855e7` feat `lrld-num` (PR #701)
  - `4f0a1c6` 2025-07-26 *"feat: add --watch flag for automatic config reloading"* (PR #1707)
  - `bf57c88` 2025-07-28 *"feat: add TCP commands for live configuration reload"* (PR #1714) — first in v1.10.0
  - `1fcac5b` 2025-08-20 *"fix: remove --watch feature due to reliability issues"* (PR #1747) — TCP reload declared preferred
  - `63069a5` 2025-12-21 *"feat(tcp): add hello, status, reload wait+timeout"* (PR #1882) — first in v1.11.0
- GitHub release notes via `gh` for **v1.10.0**, **v1.10.1**, **v1.11.0**, **v1.12.0** (each confirming the reload-related changelog bullets above).

## Blocker / open questions for kanta

- **No kanta-side pseudocode yet** — the call shape is specified above. The
  main UX-level decision is whether kanta ships *only* the TCP-reload path
  (requires `kanata >= v1.10.0`) or also implements the *"kill and re-exec
  kanata"* fallback for users on older kanata or runs without `-p`. The
  fallback implies a service-manager / port-rebinding / uptime-blip UX that
  should not be the default path.
- **JSON protocol dependency** — kanta will embed the `kanata_tcp_protocol`
  message enum (`tcp_protocol/src/lib.rs`) or hand-roll JSON. Hand-rolling
  the three reload messages is only ~5 lines per variant; embedding the
  crate adds a `kanata-tcp-protocol = "1.x"` dep but is the more honest
  mirror. Decision is implementation-time, not blocker.
