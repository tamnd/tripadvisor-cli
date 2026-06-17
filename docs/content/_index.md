---
title: "ta"
description: "A command line for TripAdvisor."
heroTitle: "tripadvisor, from the command line"
heroLead: "A command line for TripAdvisor. One pure-Go binary, two planes that share one location id, output that pipes into the rest of your tools, and a resource-URI driver other programs can address."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

`ta` reads public TripAdvisor data over plain HTTPS, shapes it into clean
records, and gets out of your way.

```bash
ta search "eiffel tower"   # find a location by name
ta location 188757         # one location as a record
ta reviews 188757          # its reviews, one per line
ta serve --addr :7777      # the same operations over HTTP
```

It reads two planes that share one `location_id` and one record shape: the web
plane on `www.tripadvisor.com` (the default, best-effort behind DataDome) and
the Content API on `api.content.tripadvisor.com` (reliable, on when
`TRIPADVISOR_API_KEY` is set). A web read and an API read address the same
record. Output adapts to where it goes: an aligned table on your terminal, JSONL
the moment you pipe it somewhere.

## Two ways to use it

- **As a command** for reading TripAdvisor by hand or in a script. Start with
  the [quick start](/getting-started/quick-start/).
- **As a resource-URI driver** so a host like
  [ant](https://github.com/tamnd/ant) can address TripAdvisor as
  `tripadvisor://` URIs and follow links across sites. See
  [resource URIs](/guides/resource-uris/).

Both are the same code: one operation, declared once, is a CLI command, an HTTP
route, an MCP tool, and a URI dereference.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Doing a specific job? The [guides](/guides/) are task-first.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
