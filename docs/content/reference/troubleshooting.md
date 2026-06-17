---
title: "Troubleshooting"
description: "The handful of things that trip people up, and how to fix each one."
weight: 40
---

Most of these come down to network reality or how TripAdvisor serves its data,
not a bug.

## A web read returns exit 4 (blocked)

`www.tripadvisor.com` is fronted by DataDome. From a datacenter or a flagged IP,
a read hits a CAPTCHA interstitial and `ta` reports it honestly with exit 4
rather than working around it. Two ways forward:

- Set `TRIPADVISOR_API_KEY` and let `--plane auto` use the Content API, which is
  reliable from anywhere.
- Run from a residential or mobile connection, where the web plane reads.

`ta` does not solve anti-bot challenges, forge sensors, or rotate proxies. A wall
is reported, not bypassed.

## A command needs a key

`nearby` is API-only, and `--plane api` forces the API plane. Either needs
`TRIPADVISOR_API_KEY` set; without it you get exit 4. Set the key, or use the web
plane for the commands that support it. See
[configuration](/reference/configuration/).

## Requests start failing or returning 429

TripAdvisor rate-limits like any public site. `ta` already paces requests and
retries the transient failures, but a hard limit still means backing off. Raise
the delay between requests with `--rate` (for example `--rate 1s`) and retry
later. A burst of 429 or 5xx responses is the site asking you to slow down, not a
defect.

## Nothing is found for something you expected

The public surface is not the whole site, and the two planes differ. The web
typeahead is best-effort; the Content API returns the fuller search. If a search
comes up short on the web plane, set a key and try the API plane. Check that the
input is spelled the way the site uses it and try a broader query.

## Stale data

Reads are cached on disk and stay fresh for 24h by default. Use `--refresh` to
fetch fresh copies and rewrite the cache, `--no-cache` to bypass it entirely, or
`--cache-ttl` to change how long a hit stays fresh.

## The binary is not on your PATH

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`), and
a release archive leaves it wherever you unpacked it. If your shell cannot find
`ta`, add that directory to your `PATH`. See
[installation](/getting-started/installation/).

## Seeing what ta actually did

When something behaves unexpectedly, `-v` adds per-request detail so you can see
the URLs it hit and the responses it got. That is usually enough to tell a rate
limit or a wall apart from a genuinely empty result.
