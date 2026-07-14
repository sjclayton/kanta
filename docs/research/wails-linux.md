# Research: Wails on Linux (MVP day-one)

**Ticket:** [sjclayton/kanta#4](https://github.com/sjclayton/kanta/issues/4)
**Date:** 2026-07-14
**Wails version cited:** **v2.13.0** (stable channel; v3 is still alpha as of `v3.0.0-alpha2.117`, 2026-07-08)

---

## TL;DR

- **Use Wails v2.13.0** (not v3) for MVP day-one. v3 is alpha-quality with an unstable API and an unfinished GTK4/WebKitGTK 6.0 backend that explicitly disclaims Linux stability.
- **WebView backend**: GTK3 + WebKit2GTK. On Linux day-one: ABI **4.1** (Debian 12+, Ubuntu 22.04+, Fedora 40+, Arch, openSUSE). Use the build tag `-tags webkit2_41`. ABI 4.0 fallback build (`-tags webkit2_40`) exists for RHEL/CentOS/Alma 8-9 and Debian 11 / Ubuntu 20.04 — already out-of-scope for the map's Win/mac posture so we don't widen Linux scope to those.
- **Wayland**: works out of the box in v2.13.0 via GTK's Wayland windowing backend. Two practical caveats: (a) KDE Wayland needs a `.desktop` file for an icon in the task switcher, and (b) Wayland's max-size constraint behaviour is fixed but remains fluttery under NVIDIA — same NVIDIA-specific env workaround as X11.
- **Single static Go binary**: **NOT possible on Linux with Wails**. Wails apps link dynamically against `libgtk-3` and `libwebkit2gtk-4.x` — these must be installed on the user's machine. The map's locked "single static binary downloaded from GitHub Releases" needs to be *reinterpreted* (recommended: tarball + small install-deps script) or *partially relaxed* for Linux (recommended: ship a tarball; the *user entry point* is still one download).
- **Build matrix**: Ubuntu 24.04 LTS + Fedora 41 + Arch are the supported smoke targets. (Debian 12 LTS works equivalently.) Add Nix to the doc-matrix as a known-good community path.

## What Wails actually is on Linux

Wails v2 boots the user's windowing system through GTK3, embeds a webview via the system `libwebkit2gtk-4.x`. Go code and JavaScript talk over an in-process IPC surface (`runtime.*` JS↔Go). Linux options ([`v2/pkg/options/linux`](https://wails.io/docs/reference/options#linux)) are minimal: `Icon`, `WindowIsTranslucent`, `WebviewGpuPolicy`, `ProgramName`. There is **no WebKitGTK-port flag** — what gets linked is decided at *build time*, via Go build tags.

## Recommended setup

### Toolchain

- Go 1.23+ (per [Wails install docs](https://wails.io/docs/gettingstarted/installation)).
- Node.js 20.x LTS + npm 10.x (per Wails default React+TS template). The map already locked **React + TS** as the frontend flavour.
- GTK3 + WebKit2GTK 4.1 development headers.

### Build commands

```
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
wails doctor              # must be all-green before continuing
wails dev                 # iterate
wails build -tags webkit2_41 -clean
```

`-tags webkit2_41` is **mandatory for MVP day-one**. Without it, Wails v2.13.0 builds against WebKit2GTK 4.0 — apt/dnf on every modern distro now ships 4.1, so the 4.0 link will fail at compile time.

### Per-distro dev dependencies

| Distro | Install |
|---|---|
| Ubuntu 24.04 / Debian 12 / Pop!_OS / Mint | `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config npm` |
| Fedora 40 / 41 / Rawhide | `sudo dnf install gtk3-devel webkit2gtk4.1-devel gcc-c++ pkgconf-pkg-config npm` |
| Arch / Manjaro / Endeavour | `sudo pacman -S gtk3 webkit2gtk-4.1 base-devel pkgconf npm` |
| openSUSE Leap 15.5+ / Tumbleweed | `sudo zypper install gtk3-devel libwebkit2gtk-4_1-devel gcc-c++ pkgconf-pkg-config npm` |
| NixOS (unstable) | `nix-shell -p pkg-config gtk3 libwebkit2gtk-4.1 go nodejs gcc` |
| Ubuntu 22.04 LTS | same as 24.04 line above; ABI 4.1 is in main repos |
| RHEL/Alma/Rocky/CentOS 8-9, Debian 11, Ubuntu 20.04 | **out of MVP Linux scope**: ABI 4.0 only, build tag `webkit2_40` — mention as known-unverified, not a smoke target |

### Runtime dependencies for end users

The kanta binary links dynamically — end users need:

| Distro | Install |
|---|---|
| Ubuntu 22.04+ / Debian 12+ | `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0` |
| Fedora 40+ | `sudo dnf install gtk3 webkit2gtk4.1` |
| Arch / Manjaro | `sudo pacman -S gtk3 webkit2gtk-4.1` |
| openSUSE Leap / Tumbleweed | `sudo zypper install libgtk-3-0 libwebkit2gtk-4_1-0` |

Failure mode if missing: a `cannot open shared object file: libwebkit2gtk-4.1.so.0` error on launch. Verify with `ldd ./kanta | grep webkit`.

## Wayland — what to expect

Wails v2.13.0 supports Wayland via GTK's `GDK_BACKEND=wayland` default selection (or `x11` when running under XWayland). Specifics:

- **Window management**: WM-independent; GTK handles decoration. Confirmed working on GNOME Wayland and KDE Wayland per Wails bug history.
- **Window max-size constraint** — historically buggy on Wayland in early v2 versions ([#4778](https://github.com/wailsapp/wails/issues/4778) fixed). With v2.13.0 we get the fix.
- **Systray context menu** — early race that hid the parent window on Wayland ([#4775](https://github.com/wailsapp/wails/issues/4775) fixed). Not relevant for MVP (no tray planned), but worth knowing.
- **Application icon**: GNOME Wayland uses `.desktop` files for task-switcher icons; KDE Wayland will use the `Icon` option. **Plan to ship a `.desktop` file alongside the binary** (per map "Distribution" decision, this is anyway the route regardless of Wayland).
- **NVIDIA on Wayland**: known webkit2gtk unstable-render bugs mirror the X11 case. Use `WEBKIT_DISABLE_DMABUF_RENDERER=1` env var (`WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand` is less reliable here). Document this in the kanta README.

**No XWayland-only degradation needed.** Modern desktops pick Wayland by default and kanta works there. We do not need to detect or branch on the user's `WAYLAND_DISPLAY`.

## Single-static-binary contradiction with the map decision

Map #1 destination locks "single static binary downloaded from GitHub Releases" as the MVP distribution. **This is impossible on Linux with Wails** — the Go binary links dynamically to system libraries that vary by distro. Concretely:

```
$ ldd ./kanta | head
  libgtk-3.so.0 => /lib/x86_64-linux-gnu/libgtk-3.so.0
  libwebkit2gtk-4.1.so.0 => /lib/x86_64-linux-gnu/libwebkit2gtk-4.1.so.0
  libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
  ...
```

Resolution, ordered by recommendation:

**Recommended**: keep "single static binary" as a *philosophy* (one download does it all) but reinterpret it as **tarball containing Go binary + install-deps shell script + `.desktop` file + LICENSE + README**. The user downloads one `kanta-v0.1.0-linux-amd64.tar.gz`, untars, runs `./install.sh` (idempotent: detects distro, runs the apt/dnf/pacman command, drops `.desktop` into `~/.local/share/applications/`, copies the binary into `~/.local/bin/`). Then `kanta` works.

**Acceptable alternative**: ship an additional `.deb` and `.rpm` from the same GitHub Release (via `nfpm` integration in GoReleaser), in addition to the tarball. Cross-distribution is then cleanest.

**Reject**: stick to a literal static binary. Would mean either ditching Wails (back to Tauri? to Gio? to walk back the charted UX investment) OR stuffing GTK3/WebKit2GTK into the binary (impractical — webkit2gtk by itself is ~50MB of shared object and depends on dozens more system libs). Unrealistic.

This contradiction is worth re-confirming at grill-time before any release-pipeline ticket (#11) locks the artifact shape, because #11's deliverable depends on it.

## Recommended Wails options for kanta

```go
Linux: &linux.Options{
    Icon:              iconPNGBytes,                              // //go:embed
    WindowIsTranslucent: false,                                   // opaque window matches required diff-overlay UX
    WebviewGpuPolicy:  linux.WebviewGpuPolicyOnDemand,           // user-controlled; rare crashes on NVIDIA → fall to Never
    ProgramName:       "kanta",                                  // matches .desktop file
}
```

Plus from App-level:

```go
SingleInstanceLock: &options.SingleInstanceLock{
    UniqueId:               "c9c8fd93-6758-4144-87d1-34bdb0a8bd60", // stable UUID for kanta
    OnSecondInstanceLaunch: app.onSecondInstanceLaunch,            // surface a second-launched file at the running instance
}
```

SingleInstanceLock is important because kanta's "open a `.kbd` file" semantics interact with file-manager double-clicks — without this, double-clicking two `.kbd` files spawns two processes that race on the same prefs/state file.

## Smoke-test plan

Before tagging an MVP release, run through all of these on **Ubuntu 24.04 LTS**, **Fedora 41**, and **Arch Linux** (live systems, not containers, because WebKit2GTK rendering inside containers on Wayland is fragile).

1. `wails doctor` is all-green on each distro.
2. `wails dev` opens the dev window: stock React+TS template appears in <2s, contains expected kanta shell text. Open browser inspector on dev mode and check console shows no errors.
3. `wails build -tags webkit2_41 -clean -trimpath -ldflags="-s -w"` produces `build/bin/kanta` and `ldd ./kanta` shows webkit2gtk-4.1 link.
4. **GUI smoke**:
   - Launch `./kanta` cold on X11 (Debian stable). Window appears, renders a layer keyboard.
   - Launch `./kanta` cold on Wayland (Fedora 41 GNOME, Arch KDE). Window appears, switches layers on demand.
   - Drag window across both Wayland compositors (max/min constraints hold).
   - Open a `.kbd` file with explicit File menu and via CLI arg `./kanta foo.kbd` — opens, layers parsed, alias list populated.
   - Edit a row in the layer keyboard and Save & Apply; `kanata --check` returns 0; TCP reload succeeds (linked decision).
5. **External-edit race**: while kanta is open, run `echo '...'` into the file via another shell → kanta's filesystem watcher reloads the model (linked ticket #12).
6. **Single-instance**: open kanta twice via two clicks of a `.kbd` → second instances routes to the first (window gains focus, opens the file).
7. **Resource pressure**: open a representative user fixture (~30 layers, ~80 keys per layer, ~50 aliases). Window remains responsive; layer switcher dropdown is snappy. Watch `/usr/bin/time -v ./kanta` baseline vs fixture load.
8. **No-GPU fallback**: on a hypothetical headless test rig or a virtualised VM, force `WebviewGpuPolicyNever` and confirm the window still renders (text only, no animation loss).
9. **CI matrix**: a non-Wayland CI image (Debian + Xvfb) builds and runs a smoke `wails build` for cross-compile confidence — but never claim release-readiness from CI alone.

## Open questions that block chart-lock on Linux specifically

These are surfaced, not resolved, in this research:

1. **Distribution artifact shape** — single-binary reinterpretation vs tarball-with-install-script vs distro packages. Feeds the Release-pipeline ticket (#11).
2. **NVIDIA/Tuxedo/Optimus hardware matrix** — out of MVP scope (we test the *majority* Linux install case; users with discrete NVIDIA + Wayland get a documented workaround).
3. **`.desktop` icon policy and icon installation** — what icon size variants kanta should ship. (Subject of `first-run UX` ticket #13's sub-decisions; this ticket only notes it's needed.)
4. **Dark/light theme glue** — does GTK theme follow the user's desktop setting? (Default-yes via GTK; subject of the `Theme specifics` ticket in fog.)

None of these block installing Wails v2.13.0 to test it on Linux today; they feed later, smaller tickets.

## What this implies for downstream tickets

- **Task: scaffold Go module + Wails project (#5)** — wins this: pick `wails@v2.13.0`, set up `internal/parser` skeleton per the #3 strategy, React+TS frontend template from `wails init -n kanta -t react-ts`. Use `-tags webkit2_41`. Ship the agreed install-deps line for the dev environment in scripts.
- **Task: GitHub Releases pipeline (#11)** — picks up the "single static binary contradiction" above and locks the artifact shape (tarball recommended).
- **Grilling: first-run template bundle contents (#13)** — accompanied by `.desktop` install question; bundle contents depend on what icon strategy is used.
- **Grilling: prefs (#10)** — `kanata binary path` preference is already locked. Add: theme (light/dark/system), icon-source path (deferred to #13). No new build implications.

## Verdict

Code-Ready: **`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0` + the distro commands above + `wails build -tags webkit2_41 -clean -trimpath -ldflags="-s -w"`**.

Decision-Ready: **single-static-binary interpretation** is not yet chart-lockable; resolve via a follow-up grilling ticket (`Grilling: Linux distribution artifact shape`) before #11 hits the build pipeline. **Everything else can be coded against**.

## Appendix — citations

- Wails release feed: https://github.com/wailsapp/wails/releases — v2.13.0 stable; v3.0.0-alpha2.117 latest pre-release (2026-07-08).
- Wails installation docs (Linux dependencies): https://wails.io/docs/gettingstarted/installation
- Wails Linux distro support (current Debian/Fedora/Arch/RHEL split): https://wails.io/docs/guides/linux-distro-support
- Wails options (Linux struct): https://wails.io/docs/reference/options#linux
- Wails apt fixture (build-time package names): https://github.com/wailsapp/wails/blob/v2.13.0/v2/internal/system/packagemanager/apt.go
- Wails bug history (Wayland signal): #4778 max-size fixed; #4775 systray context menu fixed; #5053 KDE icon notes (open); #5295 NVIDIA X11 workaround; #4870 Ubuntu 25 systemd black window; #4957 v3 GTK4/WebKitGTK 6.0 experimental.
