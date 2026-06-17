---
title: "Resource URIs"
description: "Use ta as a database/sql-style driver so a host program can address TripAdvisor as tripadvisor:// URIs."
weight: 20
---

`ta` is a command line, but the `tripadvisor` Go package is also a small driver
that makes TripAdvisor addressable as a resource URI. A host program registers it
the way a program registers a database driver with `database/sql`, then
dereferences `tripadvisor://` URIs without knowing anything about how TripAdvisor
is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behaviour.

## Mounting the driver

A host enables the driver with one blank import, exactly like `import _
"github.com/lib/pq"`:

```go
import _ "github.com/tamnd/tripadvisor-cli/tripadvisor"
```

The package's `init` registers a domain with the scheme `tripadvisor` for the
host `tripadvisor.com`. The standalone `ta` binary does not change.

## Addressing records

A URI is `scheme://authority/id`. The id is the canonical `location_id` shared by
both planes:

| URI | What it is |
| --- | --- |
| `tripadvisor://location/188757` | a location (hotel, restaurant, attraction, or geo) by its id |

```bash
ant get tripadvisor://location/188757      # the location record
ant cat tripadvisor://location/188757      # just the description body
ant url tripadvisor://location/188757      # the addressable URL
ant resolve https://www.tripadvisor.com/Attraction_Review-g187147-d188757-Reviews-Eiffel_Tower.html
```

The last line resolves a pasted page link back to its `tripadvisor://location/188757`
URI offline, the same work `ta ref id` does.

## Walking the graph

Each location record carries `kit:"link"` edges, so a host can follow the graph
and write it to disk:

| Edge | Points at | Direction |
| --- | --- | --- |
| `reviews_ref` | the location's reviews | out to a list |
| `photos_ref` | the location's photos | out to a list |
| `nearby_ref` | locations near its coordinate | out to a list |
| `parent_ref` | its nearest geo (city or region) | up one step |
| `ancestor_refs` | every geo ancestor (city, region, country) | up the whole tree |
| `neighborhood_refs` | the sub-geos inside it (districts) | down the tree |

The geo edges make the tree walkable both ways from any seed. `ancestor_refs`
climbs from a place to its city, region, and country in one expansion, and
`neighborhood_refs` descends a city into the districts inside it. Each ancestor
and neighborhood is itself a `location` id, so a crawl follows them like any
other edge.

```bash
ant ls     tripadvisor://location/188757            # the edges out of this location
ant export tripadvisor://location/188757 --follow 2 --to ./data
```

`ant export --follow` and `ant graph` walk those edges, across tools when a link
points at another site's scheme.

## Reconstructing the site

The sitemap backbone plus the geo tree reach every public page. `sitemaps` reads
the per-section indexes that `robots.txt` advertises, each `sitemap` index lists
shards, and each shard enumerates landing pages as `Seed` records. A seed fills a
`location` edge whenever a `d`-number resolves from its URL, so a breadth-first
walk runs from the sitemap roots down to every location, then sideways through
`ancestor_refs` and `neighborhood_refs` to fill the geo hierarchy:

```bash
ta sitemaps                                  # the index roots, the backbone
ant export tripadvisor://sitemaps --follow 3 --to ./data
```

No record is a dead leaf: a review and a photo both link back to their location,
and a location links up to its ancestors and down to its neighborhoods, so a host
that starts anywhere can reach the rest of the public graph.

## Why this is the same code

The driver and the binary share one definition per operation. The `location` op
answers both `ta location` on the command line and `ant get
tripadvisor://location/...` through a host, from the same handler and the same
client. There is no second implementation to keep in step.
