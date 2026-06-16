# mlrshift

Surgically shift a slot LUT's **Most-Likely-Result (MLR)** by reweighting a CSV shift without adding or removing a single row.

[![Go Report Card](https://goreportcard.com/badge/github.com/mnemoo/mlrshift)](https://goreportcard.com/report/github.com/mnemoo/mlrshift)
[![Go Reference](https://pkg.go.dev/badge/github.com/mnemoo/mlrshift.svg)](https://pkg.go.dev/github.com/mnemoo/mlrshift)

---

## What it does

A LUT (look-up table) is a list of `(id, weight, payout)` rows. The **Most-Likely-Result** is the single heaviest row; its frequency is reported as **"1 in N"** (`total_weight / max_weight`).

`mlrshift` makes that heaviest result **no more frequent than 1 in N** by **pure reweighting**:

- no rows are added or removed,
- `id`s and `payout`s stay perfectly aligned,
- **only weights change.**

It does this by clamping the heaviest rows down to a cap and redistributing the freed weight into a donor band — optionally **keeping RTP locked to its original value** and verifying that it actually held. Zero third-party dependencies: it's pure Go standard library.

---

## Install

### `go install`

```sh
go install github.com/mnemoo/mlrshift@latest
```

### Download a release binary

Grab a prebuilt binary from the [releases page](https://github.com/mnemoo/mlrshift/releases). Static, CGO-free binaries are published for:

| OS | Architectures |
|---------|------------------------------------|
| Windows | `amd64`, `386`, `arm64` |
| Linux | `amd64`, `386`, `arm64`, `arm` |
| macOS | `amd64` (Intel), `arm64` (Apple Silicon) |
| FreeBSD | `amd64` |

### Build from source

```sh
git clone https://github.com/mnemoo/mlrshift
cd mlrshift
make build      # or: go build -o mlrshift .
```

`make` (or plain `go build`) produces a single self-contained `mlrshift` binary. Requires Go 1.23+.

---

## CSV format

A **headerless** CSV with exactly three unsigned-integer columns:

```
id,weight,payoutMultiplier
```

- **`id`** — the row identifier (kept aligned, never reordered).
- **`weight`** — the row's weight (an integer; this is the only field `mlrshift` changes).
- **`payoutMultiplier`** — the payout multiplier **in CENTS**: `100` = `1.00x`, `27` = `0.27x`, `0` = a losing (0x) outcome.

Example:

```
1,3116448581,0
2,3116448581,0
601,17933,27
640,5,500000
```

That last row is a `5000.00x` outcome (`500000` cents).

---

## Quickstart

### `magic` — the easy button (the default)

Give it **only a target MLR**. `magic` picks the donor that reaches the target with the smallest RTP disturbance and no overflow, then applies `--rtp-lock` for you — so RTP is pinned to its original value automatically.

`magic` is the **default command**, so the word is optional — these two are identical:

```sh
mlrshift testdata/base.csv --target-n 250
mlrshift magic testdata/base.csv --target-n 250
```

```
MLR magic: testdata/base.csv
─────────────────────────────────
Rows:           1015
Target:         1 in 250
Auto donor:     0-1.00   (chosen automatically for the smallest RTP disturbance)
Most likely:    1 in 174.26 @ 0.27x   ->   1 in 250.00 @ 0.27x
RTP:            0.970000 (97.00%)   ->   0.970000 (97.00%)   [Δ +0.0000 pp]
RTP lock:       ✓ held — RTP pinned to 0.970000 (Δ +0.0000 pp)
(dry run — pass --save to overwrite the CSV)
```

The MLR went from **1 in 174.26** to exactly **1 in 250**, RTP is unchanged, and the lock is verified **✓ held**. By default this is a **dry run** — add `--save` to overwrite the CSV in place.

### `inspect` — preview options (read-only)

See the current MLR, which values are driving it above target, and a preview of every donor option with its resulting MLR / RTP delta:

```sh
mlrshift inspect testdata/base.csv --target-n 250
```

```
MLR inspect: testdata/base.csv
─────────────────────────────────
Rows:           1015   Unique payouts: 641
RTP:            0.970000 (97.00%)
Most likely:    1 in 174.26 @ 0.27x
Target:         1 in 250

Values driving MLR above target  (1 value(s), freed weight = 0.174%):
  payout       nrows     share%      now        uniform-within
      0.27x        1     0.574%   1 in 174.3    1 in 174.3
  (uniform-within = MLR if a value's weight were spread evenly over its OWN rows:
   free, zero RTP/structure change — but only helps if it already reaches the target)

Donor preview  (smaller |Δpp| = less RTP disturbance):
  --donor           MLR after      RTP after       Δpp        overflow
  loss             1 in 250.0     0.9695 ( 96.95%)    -0.047    0.000%
  spread           1 in 250.0     0.9700 ( 97.00%)    +0.000    0.000%
  0.27-0.54        1 in 249.6     0.9712 ( 97.12%)    +0.122    0.174%
  0-0.27           1 in 250.0     0.9695 ( 96.95%)    -0.047    0.000%
  0-1.00           1 in 250.0     0.9700 ( 97.00%)    +0.000    0.000%

Recommended: --donor 0-1.00  (hits 1 in 250, smallest RTP shift, no overflow)
Add --rtp-lock to snap RTP back to 0.9700 exactly (compensates via the tail).
Apply:  mlrshift shift 'testdata/base.csv' --cost 1 --target-n 250 --donor 0-1.00 --rtp-lock --save
```

`inspect` ends with a ready-to-run `shift` command using its recommended donor. (`magic` is exactly this recommendation, applied for you with the lock on.)

### `shift` — apply a specific donor

Run the exact reweight, choosing the donor yourself:

```sh
mlrshift shift testdata/base.csv --target-n 250 --donor 0-1.00 --rtp-lock
```

```
MLR shift: testdata/base.csv
─────────────────────────────────
Rows:           1015
Most likely:    1 in 174.26 @ 0.27x   ->   1 in 250.00 @ 0.27x
RTP:            0.970000 (97.00%)   ->   0.970000 (97.00%)   [Δ +0.0000 pp]
RTP lock:       ✓ held — RTP pinned to 0.970000 (Δ +0.0000 pp)
Weight moved:   0.1739% of total  (donor: 0-1.00)
Note:           0.0000% could not fit the donor (its rows hit the cap); total shrank, RTP shifted further.
(dry run — pass --save to overwrite the CSV)
```

#### Donor options (`--donor`)

The donor is where the freed weight gets deposited:

| `--donor` | Meaning |
|------------------|------------------------------------------------------------------|
| `loss` | deposit into `0x` rows only — RTP can only go **down** (default) |
| `spread` | deposit into **every** row with headroom, proportional to it |
| `<minX>-<maxX>` | a raw-multiplier bucket, e.g. `1-3`, `1x-3x`, or `0-1.00` |

---

## RTP is kept locked

Hitting a target MLR is easy if you don't care about the return-to-player. `mlrshift` cares.

Pass `--rtp-lock` (or use `magic`, which sets it for you) and the tool snaps RTP back to its **exact original value** by moving a sliver of weight between the high-payout tail and the low payouts — never touching the single global-max (maxwin) payout, so maxwin frequency is preserved.

Crucially, it **verifies the result and tells you the truth** with one line:

- **`RTP lock: ✓ held`** — RTP was pinned back to its original value (drift well under `0.01 pp`).
- **`RTP lock: ✗ NOT held`** — the donor saturated or the cap blocked the correction, and RTP genuinely drifted. This is printed **loudly** so a silent RTP shift is impossible to miss:

```sh
mlrshift magic testdata/base.csv --target-n 1000
```

```
MLR magic: testdata/base.csv
─────────────────────────────────
Rows:           1015
Target:         1 in 1000
Auto donor:     0-1777.79   (chosen automatically for the smallest RTP disturbance)
Most likely:    1 in 174.26 @ 0.27x   ->   1 in 1000.00 @ 0.00x
RTP:            0.970000 (97.00%)   ->   101.780488 (10178.05%)   [Δ +10081.0488 pp]
RTP lock:       ✗ NOT held — RTP drifted +10081.0488 pp; the donor saturated / cap was reached.
                Hitting this MLR and keeping RTP are infeasible together here — lower --target-n or widen --donor.
(dry run — pass --save to overwrite the CSV)
```

This is the honest **"pick two of three"** limit: a target MLR, RTP preservation, and reweight-only can't all hold when one outcome dominates the weight. `magic` always gives you a clear, verified report either way — it never silently moves RTP. See [`docs/ALGORITHM.md`](docs/ALGORITHM.md) for the full analysis.

---

## `--save` vs dry-run

Every `shift` and `magic` is a **dry run by default**: it computes and reports the result but does **not** touch your CSV. Add `--save` to overwrite the file in place (written atomically via a temp file + rename, so an interrupted write never truncates the original):

```sh
mlrshift magic testdata/base.csv --target-n 250 --save
```

`inspect` is always read-only.

---

## `--json` mode

Every command accepts `--json` for machine-readable output:

```sh
mlrshift magic testdata/base.csv --target-n 250 --json
```

```json
{
  "file": "testdata/base.csv",
  "already_met": false,
  "donor": "0-1.00",
  "feasible": true,
  "rtp_lock_held": true,
  "saved": false,
  "rows": 1015,
  "odds_before": 174.25913019909365,
  "odds_after": 250.00000003825,
  "rtp_before": 0.9699999441826498,
  "rtp_after": 0.9699999441833796,
  "rtp_delta_pp": 7.298606163885779e-11,
  "leftover_pct": 1.000000005596e-10
}
```

---

## Common flags

| Flag | Commands | Meaning |
|----------------|------------------------|--------------------------------------------------------------|
| `--target-n N` | `magic`, `shift`, `inspect` | MLR must be no more frequent than `1 in N`. Required for `magic`/`shift`; defaults to `1000` for `inspect`. |
| `--donor D` | `shift` | donor band: `loss` \| `spread` \| `<minX>-<maxX>` (default `loss`). |
| `--rtp-lock` | `shift` | snap RTP back to its original value exactly (always on in `magic`). |
| `--cost C` | all | mode cost, used for RTP reporting only (default `1`). |
| `--save` | `magic`, `shift` | overwrite the CSV in place (default: dry run). |
| `--json` | all | machine-readable JSON output. |

Run `mlrshift <command> --help` for the full per-command flag list, or `mlrshift help` for the command overview.

---

## License

[MIT](LICENSE) © 2026 Mnemoo
