### `rate` — the session outcome, in one number

```sh
fathom rate <1-5> [--note "..."] [--repo .] [--session S]
fathom rate --list
```

Appends to the local ratings log — the outcome half of the measurement
loop (`observe` files the specific friction; `rate` files the verdict).
**Experimental**: an optional human override; the A/B criterion cannot be
evaluated until ratings exist.
