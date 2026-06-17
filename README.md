# ta

A command line for TripAdvisor.

`ta` is a single pure-Go binary. It reads public TripAdvisor data over plain
HTTPS, shapes it into clean records, and prints output that pipes into the rest
of your tools.

It reads two planes that share one location id and one record shape:

- The **web plane** reads `www.tripadvisor.com` and is the default. It is
  fronted by DataDome, so from a datacenter it is best-effort and a wall returns
  exit 4. From a residential or mobile connection it reads.
- The **Content API plane** reads `api.content.tripadvisor.com` reliably from
  anywhere. It turns on when `TRIPADVISOR_API_KEY` is set in the environment.
  The key is free from TripAdvisor and is read from the environment only, never
  a flag.

Because both planes carry the same `location_id` and the same record fields, a
web read and an API read address the same record. Set the key and the same
commands get reliable, or leave it unset and they still work from a residential
connection.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
TripAdvisor as `tripadvisor://` URIs.

## Install

```bash
go install github.com/tamnd/tripadvisor-cli/cmd/ta@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/tripadvisor-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/ta:latest --help
```

## Usage

```bash
export TRIPADVISOR_API_KEY=...        # free key, optional, makes reads reliable

ta search "eiffel tower"              # find a location by name
ta location 188757                    # one location as a record
ta location 188757 -o json           # as JSON, ready for jq
ta reviews 188757                     # its reviews, one per line
ta photos 188757                      # its photos, with image URLs
ta nearby --lat-long 48.85,2.29      # locations near a point (API plane)
ta sitemap attractions -n 50         # enumerate pages from the crawl backbone
ta ref id <url>                       # resolve any TripAdvisor URL to its id
ta --help                             # the whole command tree
```

Every command shares one output contract:
`-o table|markdown|json|jsonl|csv|tsv|url|raw`, `--fields` to pick columns,
`--template` for a custom line, and `-n` to limit. The default adapts to where
output goes (a color-aware table on a terminal, JSONL in a pipe), so the same
command reads well by hand and parses cleanly downstream.

Pick a plane with `--plane web|api|auto` (default `auto`: the API when a key is
set, the web otherwise). The `ref` commands are offline and resolve URLs to ids
and back with no network at all.

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
ta serve --addr :7777    # GET /v1/location/188757  returns NDJSON
ta mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`ta` registers a `tripadvisor` domain the way a program registers a database
driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/tripadvisor-cli/tripadvisor"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `tripadvisor://` URIs without knowing anything about TripAdvisor:

```bash
ant get tripadvisor://location/188757   # fetch the record
ant cat tripadvisor://location/188757   # just the description body
ant ls  tripadvisor://location/188757   # the edges (reviews, photos, parent)
ant url tripadvisor://location/188757   # the addressable URL
```

## Attribution

TripAdvisor's terms ask that displays of its content carry the source link and
the rating image. Every record keeps its `web_url` and `rating_image_url`, so a
downstream view can comply. `ta` reads public, read-only data only. It does not
log in, store credentials, or solve anti-bot challenges, and it is honest about a
walled surface rather than working around it.

## Development

```
cmd/ta/       thin main: hands cli.NewApp to kit.Run
cli/          assembles the kit App from the tripadvisor domain
tripadvisor/  the library: HTTP client, two-plane readers, data models, and domain.go (the driver)
docs/         tago documentation site
```

```bash
make build      # ./bin/ta
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

`ta` is an independent tool and is not affiliated with TripAdvisor. Apache-2.0.
See [LICENSE](LICENSE).
