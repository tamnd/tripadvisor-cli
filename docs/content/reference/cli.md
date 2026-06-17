---
title: "CLI"
description: "Every command and subcommand, with the flags that matter."
weight: 10
---

```
ta <command> [arguments] [flags]
```

Run `ta <command> --help` for the full flag list on any command.

## Read commands

| Command | What it does |
|---|---|
| `search <query>` | Search locations by name (API when keyed, best-effort web typeahead otherwise) |
| `location <ref>` | Show one location in full (web island by URL, API details by id) |
| `reviews <ref>` | List a location's reviews |
| `photos <ref>` | List a location's photos (API plane; the web island carries only the hero image) |
| `nearby <latlong>` | List locations near a point (API plane only; needs `TRIPADVISOR_API_KEY`) |

A `<ref>` is a numeric location id (the `d`-number in a page URL), a `g`-number
for a geo, or a full TripAdvisor URL.

## Crawl commands

| Command | What it does |
|---|---|
| `sitemaps` | List the sitemap indexes TripAdvisor advertises in robots.txt |
| `sitemap <kind>` | Enumerate a kind's pages from its sitemap index (the crawl root) |

## Ref commands (offline)

| Command | What it does |
|---|---|
| `ref id <url>` | Resolve a URL, path, or bare id to its canonical (kind, id) |
| `ref url <kind> <id>` | Build the addressable URL for a (kind, id) |

These need no network and answer instantly.

## Other commands

| Command | What it does |
|---|---|
| `serve [--addr]` | Serve the operations over HTTP as NDJSON |
| `mcp` | Run as an MCP server over stdio |
| `version` | Print the version and exit |

## Plane and search flags

| Flag | Meaning |
|---|---|
| `--plane` | Which plane to read: `web`, `api`, or `auto` (default `auto`) |
| `--category` | Search category: `hotels`, `restaurants`, `attractions`, or `geos` |
| `--lat-long` | Bias search to a point as `"lat,long"` |
| `--radius` | Search radius around `--lat-long` |
| `--radius-unit` | Radius unit: `km` or `mi` |
| `--language` | Language for the API and the web `hl` (default `en`) |
| `--currency` | ISO 4217 currency for price fields (default `USD`) |

The Content API key is read from `TRIPADVISOR_API_KEY` in the environment, never
a flag. See [configuration](/reference/configuration/).

## Global flags

These are shared by every operation, so they work the same on every command.

| Flag | Meaning |
|---|---|
| `-o, --output` | Output format: `auto`, `table`, `markdown`, `json`, `jsonl`, `csv`, `tsv`, `url`, `raw` |
| `--fields` | Comma-separated columns to keep |
| `--template` | Go text/template applied per record |
| `--no-header` | Omit the header row in `table` and `csv` |
| `-n, --limit` | Stop after N records (0 means no limit) |
| `--user-agent` | User-Agent sent with each request |
| `--rate` | Minimum delay between requests |
| `--retries` | Retry attempts on rate limit or 5xx |
| `--timeout` | Per-request timeout |
| `--cache-ttl` | How long a cached response stays fresh |
| `--no-cache` | Bypass on-disk caches |
| `--refresh` | Fetch fresh copies and rewrite the cache, ignoring any hit |
| `--data-dir` | Override the data directory |
| `--db` | Tee every record into a store (e.g. `out.db`, `postgres://...`) |
| `-v, --verbose` | Increase verbosity (repeatable) |
| `-q, --quiet` | Suppress progress output |
| `--color` | `auto`, `always`, or `never` |

See [output formats](/reference/output/) for what `-o`, `--fields`, and
`--template` produce, and [configuration](/reference/configuration/) for
environment variables and defaults.
