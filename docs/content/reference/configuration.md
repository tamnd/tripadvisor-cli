---
title: "Configuration"
description: "Environment variables, the API key, defaults, and the data directory."
weight: 20
---

`ta` needs almost no configuration: it runs anonymously against public data out
of the box. The settings below let you turn on the Content API plane and tune
politeness and storage.

## The Content API key

Set `TRIPADVISOR_API_KEY` to read the Content API plane, which is reliable from
anywhere:

```bash
export TRIPADVISOR_API_KEY=...
```

The key is free from TripAdvisor's developer portal. It is read from the
environment only, never a flag, so it stays out of your shell history and process
list. With no key set, `ta` reads the web plane, which works from a residential
or mobile connection but is best-effort behind DataDome from a datacenter.

`--plane` decides which plane a command uses:

- `auto` (default): the API when a key is set, the web otherwise.
- `web`: force the web plane even when a key is set.
- `api`: force the API plane; without a key this exits 4 (needs auth).

`nearby` is API-only, so it needs a key regardless of `--plane`.

## Defaults

| Setting | Default | Flag |
|---|---|---|
| Plane | `auto` | `--plane` |
| Language | `en` | `--language` |
| Currency | `USD` | `--currency` |
| Requests | paced and retried on 429/5xx | `--rate`, `--retries` |
| Per-request timeout | 30s | `--timeout` |
| On-disk cache | under the data directory, fresh for 24h | `--cache-ttl`, `--no-cache`, `--refresh` |

## The data directory

Caches and any record store live under one data directory, chosen in this order:

1. `--data-dir`
2. `TA_DATA_DIR`
3. `$XDG_DATA_HOME/ta`
4. `~/.local/share/ta`

## Environment variables

Every flag has an environment fallback, prefixed `TA_` in upper case with dashes
as underscores. For example:

```bash
export TA_RATE=1s        # same as --rate 1s
export TA_PLANE=web      # same as --plane web
export TA_DATA_DIR=~/data/ta
```

Flags win over environment variables, which win over the built-in defaults. The
one exception is the API key, which has no flag and is read only from
`TRIPADVISOR_API_KEY`.

## Sending records to a store

`--db` tees every emitted record into a store as a side effect of reading, so a
session fills a local database without a separate import step:

```bash
ta location 188757 --db out.db        # SQLite file
ta reviews 188757 --db 'postgres://...'
```
