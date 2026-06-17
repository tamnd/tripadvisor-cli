---
title: "Output formats"
description: "The output contract every command shares: formats, fields, and templates."
weight: 30
---

Every list command in the fleet renders through one formatter, so the same flags
work everywhere. Wire your commands through it as you add them, and this page
describes what users get. Pick a format with `-o`, or let ta choose:
a table when writing to a terminal, JSONL when piped.

## Formats

```bash
ta <command> -o table     # a rounded, color-aware grid for reading
ta <command> -o markdown  # a GitHub pipe table to paste into docs (alias: md)
ta <command> -o list      # one record per section, easy on the eyes
ta <command> -o jsonl     # one JSON object per line, for piping
ta <command> -o json      # a single JSON array
ta <command> -o csv       # spreadsheet friendly
ta <command> -o tsv       # tab-separated
ta <command> -o url       # just the URL column
ta <command> -o raw       # the underlying bytes, unformatted
```

| Format | Best for |
|---|---|
| `table` | Reading on a terminal: a rounded border with an accented header |
| `markdown` | Pasting into a README, issue, or PR (alias `md`) |
| `list` | Reading one record at a time: a heading and a short bullet list per record |
| `jsonl` | Piping into another tool, one object at a time |
| `json` | Loading a whole result as an array |
| `csv` / `tsv` | Spreadsheets and quick column math |
| `url` | Feeding URLs into other commands |
| `raw` | The unformatted bytes (response bodies, file contents) |

## Color

On an interactive terminal the `table`, `list`, and `json`/`jsonl` formats are
colored: the table draws a dim border with an accented header, `list` styles each
record's heading and keys, and JSON keys, strings, numbers, and literals are
highlighted. Color is suppressed the moment output is not a terminal, so a pipe
always gets plain, parseable bytes (and `list` falls back to literal Markdown).
Force the choice with `--color always|never` (or set `NO_COLOR`). `markdown`,
`csv`, `tsv`, `url`, and `raw` are never colored, so they stay safe to redirect
into a file.

## Narrowing columns

Keep only the fields you want:

```bash
ta <command> --fields id,title,url
```

`--no-header` drops the header row in `table` and `csv` output, which helps when
a downstream tool expects bare rows.

## Templating rows

For full control over each line, apply a Go text/template. Fields are the JSON
keys, capitalised:

```bash
ta <command> --template '{{.URL}} {{.Title}}'
```

## Why auto-detection helps

Because the default adapts to the destination, the same command reads well by
hand and parses cleanly in a pipe:

```bash
ta <command>            # a table, because this is a terminal
ta <command> | wc -l    # JSONL, because this is a pipe
```

You only reach for `-o` when you want something other than that default.
