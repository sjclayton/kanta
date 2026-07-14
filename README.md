# kanta

Visual layout / settings editor for [Kanata](https://github.com/jtroo/kanata).

Linux day-one. MVP target: open a `.kbd` file, edit `defsrc` / `deflayer` / `defalias` / `defcfg`
visually, leave `defchord` / `defseq` / `defcomb` / `defvolt` / `defover` preserved as text,
**Save & Apply** writes a valid config and hot-reloads the running kanata in one atomic step.

See [AGENTS.md](./AGENTS.md) for agent conventions and the project map in
[sjclayton/kanta#1](https://github.com/sjclayton/kanta/issues/1).

## Build

Requires [Wails v2.13.0](https://wails.io), Go 1.23+, Node 20+, and the system libraries
listed in [docs/research/wails-linux.md](docs/research/wails-linux.md).

```
./scripts/build.sh
```

Produces `dist/kanta-<version>-linux-amd64`.

## Develop

```
wails dev
```

Loads with hot reload. Calls into Go methods from devtools at http://localhost:34115.

## Test

```
go test ./...
```
