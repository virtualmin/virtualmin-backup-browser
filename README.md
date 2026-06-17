# virtualmin-backup-browser

`vmbb` — a small, self-contained CLI for exploring [Virtualmin](https://www.virtualmin.com)
backup archives without needing Perl, GNU tar, or the matching compression
tools installed. A single static binary (~2.5 MB) understands every container
format Virtualmin produces, including the nested "tar inside a tar" layout.

## Why

Virtualmin backups are an outer archive of per-feature files, and some of those
files (notably the home directory) are themselves archives. On Windows and
macOS the bundled archive tools often have incomplete support for GNU
tar extensions, making backups awkward to inspect. `vmbb` reads them directly.

## Supported containers

| Extension   | Container        |
|-------------|------------------|
| `.tar`      | uncompressed tar |
| `.tar.gz`   | gzip             |
| `.tar.bz2`  | bzip2            |
| `.tar.zst`  | zstd             |
| `.zip`      | zip              |

Compression is detected by magic bytes, not the filename.

## Usage

```
vmbb list <backup>              Summarise domains and features
vmbb tree <backup> [--deep]     List every member (--deep recurses nested archives)
vmbb cat <backup> <entry>       Write a member to stdout (nested: outer::inner)
vmbb extract <backup> <entry> [-o dir]   Extract a member to disk
```

A nested path uses `::` to descend one archive level, e.g.

```
vmbb cat backup.tar.gz "example.com_dir.tar::public_html/index.html"
```

## Backup layout (what vmbb parses)

Members are named `[.backup/]<domain>_<feature>[_<sub>][archive-suffix]`. The
domain never contains an underscore, so the first underscore separates it from
the feature id. Global configuration lives under the pseudo-domain
`virtualmin`. `.info`/`.dom` sidecar files (directory-format backups) use
Webmin's `serialise_variable` encoding, decoded by `internal/serialise`.

## Build

```
go build -o vmbb ./cmd/vmbb
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o vmbb.exe ./cmd/vmbb
```

## Status

Implemented: format detection, the five containers, single-file backups,
nested `_dir.tar` recursion, `list`/`tree`/`cat`/`extract`, the metadata decoder.

Not yet implemented: directory-format and home-format browsing across a
directory of per-domain archives; `info` (decoded `.dom`/`.info` display);
feature-aware categorisation output; GPG-encrypted backups; GNU
`--listed-incremental` differential metadata.
