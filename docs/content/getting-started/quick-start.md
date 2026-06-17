---
title: "Quick start"
description: "Fetch your first record with ta."
weight: 30
---

Once `ta` is on your `PATH`, set the optional key and search for a location.
The key is free from TripAdvisor and makes reads reliable from anywhere; without
it `ta` reads the web plane, which works from a residential connection:

```bash
export TRIPADVISOR_API_KEY=...
ta search "eiffel tower"
```

By default you get an aligned table. Ask for JSON when you want to pipe it:

```bash
$ ta search "eiffel tower" -o json
[
  {
    "id": "188757",
    "name": "Eiffel Tower",
    "category": "attraction",
    "city": "Paris",
    "url": "https://www.tripadvisor.com/Attraction_Review-g187147-d188757-Reviews-Eiffel_Tower.html"
  }
]
```

## Read one location

`location` takes the numeric id (the `d`-number in a page URL) and returns the
full record: rating, ranking, address, hours, and the edges into its reviews,
photos, and parent geo.

```bash
ta location 188757                    # full record as a table
ta location 188757 -o json | jq .rating
```

## Shape the output

The same flags work on every command:

```bash
ta location 188757 --fields name,rating,num_reviews   # keep only these columns
ta location 188757 --template '{{.Name}}: {{.Rating}}' # a custom line
ta reviews 188757 -o jsonl | jq .text                  # one object per line, into jq
```

`-o` takes `table`, `markdown`, `list`, `json`, `jsonl`, `csv`, `tsv`, `url`, or
`raw`. Left to `auto`, it prints a table to a terminal and JSONL into a pipe. See
[output formats](/reference/output/) for the full contract.

## Reviews, photos, and nearby

```bash
ta reviews 188757 -n 20               # the most recent reviews
ta photos 188757 -o url               # just the image URLs
ta nearby --lat-long 48.8584,2.2945   # locations near a point (API plane)
```

## Resolve any URL offline

The `ref` commands need no network. They turn any TripAdvisor URL, path, or bare
id into a canonical id and back into an addressable URL:

```bash
ta ref id "https://www.tripadvisor.com/Attraction_Review-g187147-d188757-Reviews-Eiffel_Tower.html"
ta ref url location 188757
```

## Crawl the backbone

`sitemaps` lists the published sitemap indexes, and `sitemap <kind>` enumerates
a kind's pages as seeds, each wired with its location edge:

```bash
ta sitemaps                           # every advertised index
ta sitemap attractions -n 50          # the first fifty attraction pages
```

## Serve it instead

The same operations are available over HTTP and to agents over MCP:

```bash
ta serve --addr :7777 &
curl -s 'localhost:7777/v1/location/188757'   # NDJSON, one record per line
ta mcp                                         # MCP over stdio
```

The [guides](/guides/) cover the common jobs in more depth.
