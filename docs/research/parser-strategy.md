# Research: kanta parser strategy

**Ticket:** [sjclayton/kanta#3](https://github.com/sjclayton/kanta/issues/3)
**Date:** 2026-07-14
**Kanata version cited:** `v1.12.0` (released 2026-07-05; commit on `main` of `jtroo/kanata`)
**Verdict:** Write a Go-native `.kbd` parser with span tracking. Validate semantically by shelling out to `kanata --check`. Hand-roll the TCP protocol in Go. **Do not embed `kanata-parser` as a library.**

---

## Why we have a problem at all

kanta opens a `.kbd` file and edits a *visual* representation of its clauses. The format is round-tripped:

1. read user-authored file → in-memory model (layers, aliases, settings),
2. mutate the model via UI,
3. write a valid file back **without destroying the user's comments, blank lines, alignment, or surrounding-clause order**.

Round-trip preservation is a hard requirement on the map ([#1 destination, decision 19 in chart](https://github.com/sjclayton/kanta/issues/1)). That requirement *is* what makes parser strategy a real question — not "can we read a `.kbd`?"

## What the upstream parser actually is

`kanata-parser` ([`parser/Cargo.toml`](https://github.com/jtroo/kanata/blob/main/parser/Cargo.toml)) is `0.1120.2`, license **LGPL-3.0-only**. Cargo-workspace member of `jtroo/kanata`:

```toml
members = ["./", "parser", "keyberon", "tcp_protocol", ...]
```

Public surface lives under [`parser/src/cfg/mod.rs`](https://github.com/jtroo/kanata/blob/main/parser/src/cfg/mod.rs):

```rust
pub fn new_from_file(p: &Path) -> MResult<Cfg>
pub fn new_from_str(cfg_text: &str, file_content: HashMap<String, String>) -> MResult<Cfg>
pub fn parse_cfg_raw_string(...) -> Result<IntermediateCfg>
pub struct Cfg {
    pub mapped_keys: MappedKeys,
    pub layer_info: Vec<LayerInfo>,  // { name, cfg_text, icon } — resolved, not source
    pub options: CfgOptions,         // defcfg
    pub layout: KanataLayout,        // keyberon layout — runtime artefact
    pub sequences, pub overrides, pub fake_keys, pub zippy, ...
}
pub struct LayerInfo {
    pub name: String,
    pub cfg_text: String,            // resolved action text per layer
    pub icon: Option<String>,
}
```

`cfg_text` is the **post-resolution** action text per layer — a single string. It is *not* the verbatim source of any clause, so re-emitting `Cfg.layer_info[i].cfg_text` gives you canonical-as-kanata-thinks-it text, not the user's input.

`parse_cfg_raw_string` is the only stage that still has `Vec<Spanned<SExpr>>` in hand. The downstream `populate_cfg_with_icfg` collapses it into `KanataLayout`, throwing the spans away. Span-tracked data is a transient internal type, not a stable public API.

The crate's `[lib]` section is absent → default crate-type = **`rlib` only** (no `staticlib`, no `cdylib`). To embed from Go/CGo, we'd have to patch `Cargo.toml` or use the workspace's `kanata_state_machine` `staticlib` (which pulls in keyberon too).

## Options, evaluated

### Option A — embed `kanata-parser` (Rust → C ABI → Go CGo)

- ✅ Parse/validate fidelity is *exactly* what kanata will accept at runtime.
- ✅ Error messages carry Span position info we can map back to line/column.
- ❌ **No round-trip preservation.** Output type is a resolved layout, not an editable AST with provenance. Kanta would have to *also* keep the raw file bytes and find/replace canonical text into the right spans — fragile.
- ❌ Crate-type mismatch: `kanata-parser` is rlib-only. Easiest path: depend on the upstream `kanata_state_machine` staticlib, which pulls keyberon + mio + evdev trans-tree *as link inputs*. Even as a "library" path we still need a Rust toolchain at kanta's build time.
- ❌ **LGPL-3.0-only.** Static linking forces kanta (Go) to either release under (L)GPL-compatible terms *or* ship the parser as a separately-replaceable shared object (LGPL §6). Either is workable but is a real fork-in-the-road for licensing.
- ❌ Semantic drift: kanata evolves; kanta pins to a kanata release. Upgrading kanta = bumping the Cargo dep + re-vendoring.
- ❌ Build complexity: Wails' Go build + Cargo toolchain + cbindgen + a Rust stdlib link. About double the build-time surface area.

**Net: rejected.** The round-trip failure alone is disqualifying. Locking kanta to a Rust toolchain for a feature we can implement in ~1.5 KLOC of Go is also negative.

### Option B — shell out to `kanata --check`

The `--check` flag ([`src/main_lib/args.rs`](https://github.com/jtroo/kanata/blob/main/src/main_lib/args.rs)) does parse + validate + exit. It's a strict validator and the canonical source of truth.

- ✅ Zero in-house parser code; uses the real kanata binary the user is already running.
- ✅ Stderr carries line/column-anchored errors we can extract from `--debug` output (kanata logs spans).
- ❌ **No parse tree at all** → we still need some in-process understanding of the file to drive the UI (which keys are in `defsrc`? what aliases are referenced in layer 3's key 12?).
- ❌ Spans round-trip through parse, not save: save re-emits a *formatted* file, dropping verbatim preservation (same as Option A) unless we re-implement a span-preserving editor on top.
- ❌ Latency / process spawn per Save; mild but real.
- ❌ Forces kanta users to keep a matching kanata binary on `$PATH` (already a requirement for Save & Apply anyway → neutral).

**Net: useful as the *semantic validator*, not as the editor model source.**

### Option C — write a parser in Go (recommended)

.kbd's syntax is a small subset of S-expressions: `(defsrc …)`, `(deflayer name …)`, `(defalias name action)`, `(defcfg …)`, plus the literal atoms that resolve to actions/keys. The action mini-language inside each `(deflayer …)` slot is also small (modifiers, key lists, taps, holds, macros, layer-switch, transparent, custom-action calls, deflayer references).

A Go-native parser covering `defsrc`, `deflayer`, `defalias`, `defcfg` covers the map's locked **Config surface (visually edited)** ([#1 decision](https://github.com/jclo.../issues/1), charted at 19). Everything else (`defchord`, `defseq`, `defcomb`, `defvolt`, `defover`) we treat as opaque text blocks preserved verbatim — no parsing required.

The lexer is a hand-rolled byte scanner (parens, quoted atoms, comments `;`, whitespace). Targets ~250 LOC.
The parser is recursive descent over `Vec<SExpr>` → top-level clauses. Targets ~600 LOC.
Each clause node carries `Span { Start, End ByteOffset, Line, Column }` — bytes retained from the source file, so edits are span text replacements into the original buffer.

Validation: on Save & Apply, kanta writes the (preserved-format) buffer to disk, then shells out to `kanata --check --cfg <path>`. On non-zero exit (or stderr error markers), we abort the save and surface line/column to the UI. The UI already requires `per-instance visual indication across the UI wherever the error is, with clear "what went wrong, why"`.

This combined strategy:
- ✅ **Format preservation**: native. Each clause is a `(start_byte, end_byte)` slice into the original source; defchord/defseq/etc. are kept as opaque ranges that kanta never modifies. Save = splice canonical re-emission for the edited spans, leave the rest alone.
- ✅ **Visual model coverage**: every `deflayer` row keyed by `(layer_name, slot_index)`, every `defalias` row keyed by `name`. UI state machine-friendly.
- ✅ **LGPL clean**: we don't *link* against `kanata-parser`. We could safely license kanta MIT/Apache/anything.
- ✅ **Build speed**: pure Go. No Rust toolchain.
- ✅ **Round-trip safety**: trivially verifiable with snapshot tests against user fixtures.
- ⚠️ **Validation fidelity**: depends on `kanata --check` for the action resolution rules. We do *not* reimplement action-mini-language semantics — that's where we'd drift. By deferring to `kanata` for validation, our model can be lighter: "I parsed your file structure; here's what I think the actions *should* be syntactically" — but `kanata --check` is the source of truth on semantics.
- ⚠️ **No in-process error spans from kanata** beyond `kanata --check`'s stderr. Fine for Save & Apply, possibly not fine for *typing*-time validation in the UI — a future ticket (probably "live validation via inotify on a scratch file") can address that without changing the parser strategy.

**Net: recommended.**

## Recommended approach (call shape)

Three Go packages inside kanta:

1. **`internal/parser`** — tokeniser + S-expr → clause tree. Exposes:
   ```go
   type Clause struct {
       Kind     ClauseKind   // Defsrc | Deflayer | Defalias | Defcfg | Opaque
       Name     string       // layer name / alias name; "" if not applicable
       Span     Span         // byte range in the original source
       Children []*Clause    // for Opaque: sub-tokens? or just bytes? — see below
       // for Deflayer:
       Rows     []ActionRef  // one per defsrc slot; ActionRef = Span + parsed action
   }
   type ParseResult struct {
       Clauses  []Clause
       Defsrc   []KeyRef     // ordered list of keys (one slot per key)
       Aliases  map[string]ActionRef
       Options  CfgOptions   // defcfg, parsed
       Original []byte       // source buffer; read-only
       Problems []Problem    // structural errors (unbalanced parens, etc.)
   }
   ```
2. **`internal/parser.Edit`** — apply edits: given a desired `(layer, slot, new_action_text)`, splice the canonical action text in *only* the matching `Span`, leaving the rest of the buffer untouched. Returns the new buffer + a list of byte ranges that changed (for the audit log).
3. **`internal/validator`** — wraps `kanata --check`. Parses stderr for line/column + message, returns structured problems. HTTP-style timeout — `5s` default, configurable.

The Save & Apply pipeline is then:
```
file := read_disk()
model := parser.Parse(file)
model = UI_apply_pending_edits(model)  // in-memory, no disk round-trip
edits := model.Diff(file)              // byte ranges to splice per changed clause
file2 := splice(file, edits)           // format-preserving
write_atomic(file2, path)
validator.Check(path)                  // kanata --check
if problems: revert_to(file), surface problems, KEEP UI edits in memory
else: tcp_reload(path)                 // already locked: TCP-driven
```

## Round-trip risk profile

| Risk | Severity | Mitigation |
|------|----------|------------|
| Action mini-language semantics drift between our Go parser and kanata | **High** | Our parser is *syntactic*; semantic truth is `kanata --check`. Lose on syntax errors during typing only; lose *real* errors at Save & Apply. |
| We accidentally mutate the Opaque regions | High | Hard rule: `Opaque` clauses are byte ranges we never re-emit. Span lints in tests. |
| Comments / blank lines lost around edited clauses | Medium | Splice in-place; only the action text inside a `(deflayer ...)` row changes, the surrounding whitespace and commenters are between clauses and stay. |
| Trailing whitespace differences confuse kanata | Low | kanata tolerates it; we'll normalise inside the action text only, never on lines we don't own. |
| User file has tabs vs spaces | Low | Preserve verbatim. Splice replacement text uses the user's local indentation if known per-region, else 4 spaces. |
| New kanata feature not yet understood by our parser | Medium | The clause is opaque until parser supports it; **a future kanata feature inside `deflayer` will appear as an opaque line**, which is detectable by the user (visual: "understood at row X" vs "not understood at row X"). Opaque-in-deflayer is *not* an MVP blocker since deflayer minimum coverage is the locked chart surface; advanced items go through the **Raw** tab. |
| Parser bug causes silent semantic loss | Medium | TDD + snapshot fixtures + mandatory `kanata --check` after every save. |

## What this implies for downstream tickets

- The **TCP-protocol library strategy** item in `Not yet specified` (map #1) leans even further away from Cargo: if the same reasoning drives it (LGPL cleanness, build speed), kanta should **hand-roll the four JSON message types** from `tcp_protocol`'s [`ServerMessage`](https://github.com/jtroo/kanata/blob/main/tcp_protocol/src/lib.rs) enum verbatim — it's already a tiny serde enum, ~50 LOC of Go generated from a `//go:generate` step. Tickets that need protocol types should expect Go DTOs, not a Cargo dep.
- The **external-edit watch + conflict UX** ticket (#12) still owes us: when disk changes while kanta's model is in memory, re-parse and diff. Parsing-on-load is cheap; this strategy makes that a one-liner.
- The **pre-reload validation + safety UX** ticket (#6) owns: how `kanata --check` failures are surfaced to the user. This ticket decides the validate-via-CLI shape; #6 owns its UX.
- The **raw-text view editability + convergence rules** ticket (#7) owns: how edits made in the Raw tab are mapped back into clauses for span-based editing vs left as opaque patches the parser doesn't understand.

Out of scope for this ticket:
- The Go data model itself (lives in the parser-implementation tickets after #5 scaffold lands).
- The UI choices (live vs on-save validation — #6 territory).

## Appendix A — verbatim citations

- kanata `Cargo.toml` (workspace + `kanata_state_machine` staticlib, `kanata-parser` path dep, `kanata-tcp-protocol` path dep): https://github.com/jtroo/kanata/blob/v1.12.0/Cargo.toml
- `kanata-parser` `Cargo.toml` (license LGPL-3.0-only, version `0.1120.2`): https://github.com/jtroo/kanata/blob/v1.12.0/parser/Cargo.toml
- `parse_cfg`, `Cfg`, `LayerInfo`, `IntermediateCfg`: https://github.com/jtroo/kanata/blob/v1.12.0/parser/src/cfg/mod.rs
- `kanata --check` flag: https://github.com/jtroo/kanata/blob/v1.12.0/src/main_lib/args.rs (see `--check` field on `Args`)
- `tcp_protocol` messages (re-implementable in Go without Cargo): https://github.com/jtroo/kanata/blob/v1.12.0/tcp_protocol/src/lib.rs

## Appendix B — decision log

This document *recommends* the strategy; locking it is a follow-up grilling ticket (proposed: `Grilling: confirm Go-native parser + kanata --check validation strategy`) which converts the recommendation into a charted decision on the map.
