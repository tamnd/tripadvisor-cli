---
title: "Introduction"
description: "What ta is and how it is put together."
weight: 10
---

A command line for TripAdvisor.

`ta` is a single binary. It speaks to TripAdvisor over plain HTTPS, shapes the
responses into clean records, and gets out of your way.

## Two planes, one record

TripAdvisor is readable two ways, and `ta` reads both behind one record shape:

- The **web plane** reads `www.tripadvisor.com`. It is the default and needs no
  key. The site is fronted by DataDome, so from a datacenter a read is
  best-effort and a wall returns exit 4. From a residential or mobile connection
  it reads.
- The **Content API plane** reads `api.content.tripadvisor.com`. It is reliable
  from anywhere and turns on when `TRIPADVISOR_API_KEY` is set in the
  environment. The key is free from TripAdvisor and is read from the environment
  only, never a flag.

Both planes carry the same `location_id` and the same fields, so a web read and
an API read address the same record. `--plane web|api|auto` chooses; the default
`auto` uses the API when a key is set and the web otherwise.

## How it is built

- A **library package** (`tripadvisor`) holds the HTTP client and the typed data
  models. It paces requests, sets an honest User-Agent, and retries the
  transient failures any public site throws under load.
- A **domain** (`tripadvisor/domain.go`) declares each operation once on the
  [any-cli/kit](https://github.com/tamnd/any-cli) framework. That single
  declaration becomes a CLI command, an HTTP route, an MCP tool, and a
  resource-URI dereference.
- A thin **`cmd/ta`** hands the assembled app to `kit.Run`, which builds the
  command tree and the serve and mcp surfaces.

## One operation, four surfaces

Because an operation is surface-neutral, the same `location` you run on the
command line is also a route and a tool:

```bash
ta location 188757                       # the command
ta serve --addr :7777                    # GET /v1/location/188757
ta mcp                                   # the location tool, over stdio
ant get tripadvisor://location/188757    # the URI dereference (via a host)
```

## Scope

`ta` is a read-only client over data TripAdvisor already serves publicly. It
does not log in, store credentials, or solve anti-bot challenges, and it is
honest about a walled surface rather than working around it. Every record keeps
its source link and rating image so a downstream display can attribute the
source. That narrow scope keeps it a single small binary with no database, no
daemon, and no setup.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
