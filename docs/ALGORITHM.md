# MLR Shift — Algorithm & Soundness

This document explains exactly what `mlrshift` does to a LUT, why each step is correct, and — honestly — where the method has a hard limit. It is the authoritative reference for the behaviour of the `shift`, `inspect`, and `magic` commands.

## The object

A **LUT** (look-up table) is a list of rows `(id, weight, payout_cents)`. Two derived quantities matter:

- **Most-Likely-Result (MLR):** the single heaviest row. Its frequency is reported as **"1 in N"**, where

  ```
  N = total_weight / max_weight
  ```

  A larger `N` means the most likely outcome is rarer.

- **RTP (return to player):** the weighted-mean payout,

  ```
  RTP = Σ(weight · payout_cents) / Σ(weight) / 100 / cost
  ```

  `cost` (the mode bet cost) is used for reporting only; it scales RTP but never changes the reweighting.

**Goal:** make the MLR no more frequent than `1 in target_N` by **pure reweighting** — no rows added or removed, `id`s and `payout`s stay aligned, **only weights change.** This is the entire contract, and it is what makes the operation safe to apply to a production LUT: the outcome structure is untouched; only how often each outcome is drawn changes.

The sums `Σ(weight · payout)` can exceed `uint64` for large LUTs (total weight up to ~1e13, payout up to ~5e5 cents ⇒ products near 5e18, sums beyond), so they are accumulated in an exact 128-bit integer. Total weight itself stays in `uint64` with an explicit overflow guard.

---

## Step 1 — CLAMP (guarantee the MLR bound)

Compute the cap:

```
cap = floor(total_weight / target_N)
```

Every row whose weight exceeds `cap` is clamped down to `cap`. The weight removed by clamping is **"freed"** and carries with it a **freed RTP**:

```
freed     = Σ over clamped rows of (weight − cap)
freed_rtp = Σ over clamped rows of (weight − cap) · payout_cents
```

Because the freed weight is re-deposited in Step 2 (so the total is approximately unchanged) and **no remaining row exceeds `cap`**, the new maximum weight is `≤ cap`, which gives

```
N_after = total / max_weight ≥ total / cap ≈ target_N.
```

So after the clamp the MLR is **at least `1 in target_N`** — provided the freed weight can be re-deposited without exceeding the cap anywhere (the no-overflow condition of Step 2).

> **Why a single payout can dominate.** If one *value* (say `0.27x`) is split across many rows, no single row need be heavy. But if one value sits on a single very heavy row, clamping it frees a large mass that must land somewhere — this is the case that stresses Steps 2–3. `inspect` surfaces exactly this: its "Values driving MLR above target" table lists each offending value, its share, and its **uniform-within** odds (the MLR you'd get for free if that value's weight were spread evenly across its *own* rows — zero RTP/structure change, but only useful if that alone already reaches the target).

---

## Step 2 — DONOR (re-deposit the freed weight, RTP-neutrally)

The freed weight is deposited into a chosen **donor band**, with every receiving row still kept `≤ cap`. Eligibility depends on the donor:

| Donor | Eligible rows | Effect on RTP |
|------------------|------------------------------------------|---------------------------------|
| `loss` | `0x` rows only | RTP can only go **down** |
| `spread` | every row with headroom, ∝ headroom | smallest, broadly-spread shift |
| `[a, b]` (bucket) | rows with payout multiplier in `[a, b]` | depends on the band |

### The RTP-neutral split

The freed mass has a mean payout

```
m = freed_rtp / freed.
```

To deposit `freed` units of weight without changing the total weighted-payout (and therefore RTP), we partition the eligible donor rows by `m`:

- **LOW** = rows with `payout ≤ m`,
- **HIGH** = rows with `payout > m`.

Place `x` into LOW and `freed − x` into HIGH, distributing **each side proportionally to its headroom** (`cap − weight`). Proportional-to-headroom placement makes each side land at its **headroom-weighted mean** payout — call them `mLo` and `mHi`. We then choose

```
x = freed · (mHi − m) / (mHi − mLo).
```

With that `x`, the combined deposited mean is

```
(x · mLo + (freed − x) · mHi) / freed = m,
```

exactly — so the total weighted-payout is unchanged and **RTP is preserved**. This is algebraically exact (modulo integer rounding). It holds **whenever the band brackets `m` and has two-sided headroom**.

### When the band does not bracket `m`

If the donor band lies entirely on one side of `m` (e.g. `loss`, which is all `0x` rows, sits below any positive `m`), there is no way to keep the mean at `m`. The tool then does the **closest feasible** deposit — for `loss`, that means RTP can only be pulled *down*. This is a legitimate, intentional outcome (you may *want* RTP to drop); it is simply not RTP-neutral, and `inspect` shows the resulting `Δpp` so the cost is explicit.

### Saturation / overflow

If the donor **saturates** — every eligible row reaches `cap` before all the freed weight is placed — the unplaced **leftover** cannot be deposited. The total weight then *shrinks*, RTP moves further than the neutral split intended, and the MLR target can slightly miss (because `total` in `N = total/max_weight` shrank). The tool reports this explicitly:

- `shift` prints a `Note:` line with the leftover percentage,
- `inspect`'s donor preview has an **`overflow`** column,
- `magic` avoids overflow donors entirely when it can.

`inspect` selects the **RTP-flattest non-overflowing band** automatically: it computes the smallest upper bound `U` such that the rows in `[0, U]` have enough cumulative headroom to absorb all the freed weight, and previews `0-U` as a candidate. That band deposits the freed mass at the lowest available payouts that still fit, minimizing disturbance without overflowing.

---

## Step 3 — RTP-LOCK (optional: pin RTP back exactly)

The neutral split keeps RTP unchanged *when the band brackets `m`*. `--rtp-lock` makes RTP exact **regardless of donor**, by snapping it back to its original value through iteration.

The target is the original weighted-payout sum, `targetWP`. Each round:

1. Compute the current weighted-payout sum and its error `diff = curWP − targetWP`.
2. If `|diff| ≤ tol`, stop. (`tol = max(total, 1)·1e-9 + 1` — a sliver above integer-rounding noise.)
3. Otherwise move a sliver of weight to cancel `diff`:
   - if RTP is **too high**, drain the **above-mean** payouts and fill the **below-mean** ones;
   - if RTP is **too low**, do the reverse.
4. The drained side is reduced **∝ current weight** and the filled side **∝ headroom**, so each side's internal shape is preserved and no row is zeroed or pushed past `cap`.
5. The amount moved each round is `min(|diff|/gap, drainable, fillable)`, where `gap` is the payout distance between the two pools — so the correction can never overshoot.

Two invariants make this safe and convergent:

- **Maxwin is never touched.** The single global-max payout row is excluded from both pools, so **maxwin frequency is preserved** — a hard requirement for slot math.
- **Bounded iteration.** The loop runs at most **256 rounds** with the tolerance above, so it **always terminates.** It also breaks early when no movable mass remains on a needed side.

**Result:** when there is two-sided movable mass, the lock pins RTP to within `~1e-9` of its original value (the `--json` output shows residuals like `rtp_delta_pp: 7.3e-11`). When the cap or lost mass (overflow) blocks the correction, the residual stays large — and that is reported, not hidden (see *Verification* below).

---

## `magic` — the one-shot path

`magic` takes only `target_N`. It runs the same analysis `inspect` does, then:

1. picks the donor that reaches the target with **no overflow** and the **smallest RTP disturbance** — *identical to what `inspect` recommends*;
2. applies that donor with `--rtp-lock` on.

If **no** overflow-free donor can reach the target (a degenerate LUT where one outcome dominates), `magic` falls back to the **closest-reaching** donor (preferring those that hit the target, then least overflow, then least `|Δpp|`), marks the result infeasible, and reports the lock as **✗ NOT held**. It never silently moves RTP.

---

## Verification — the held / NOT-held line

Because RTP preservation can genuinely fail, `mlrshift` *verifies* it and reports the result with one line whenever `--rtp-lock` was requested (or always, in `magic`):

- **`RTP lock: ✓ held`** — `|Δpp| < 0.01`. RTP was pinned back to its original value.
- **`RTP lock: ✗ NOT held`** — `|Δpp| ≥ 0.01`. RTP genuinely drifted because the donor saturated or the cap was reached; the line is printed loudly with a remedy (lower `--target-n` or widen `--donor`).

The `0.01 pp` threshold sits cleanly in the gap between the two regimes: a converged lock is *many orders of magnitude* below it (`~1e-9`), while a blocked lock is far above it. There is no ambiguous middle ground in practice.

---

## Soundness verdict

The procedure is **sound**:

- it is a **mass-conserving reweight** — structure (ids, payouts, row count) is invariant by construction;
- the **cap guarantees the MLR bound** whenever there is no overflow;
- the **RTP-neutral split is algebraically exact** when the band brackets the freed mean and has two-sided headroom;
- **rtp-lock provably terminates** (256-iteration bound, monotone tolerance) and pins RTP to `~1e-9` when feasible;
- **maxwin frequency is preserved** throughout.

### The honest limitation — "pick two of three"

Three desirable properties cannot **all** hold simultaneously when **one outcome dominates the weight**:

1. hit the MLR target,
2. preserve RTP exactly,
3. reweight only (no rows added or removed).

When a single outcome carries too much weight, the cap frees a mass too large to redistribute within any RTP-neutral band that has headroom. You must then accept one of:

- **overflow** (RTP moves — property 2 yields),
- a **lower `target_N`** (property 1 relaxes), or
- **splitting the dominant books** into more rows (property 3 yields — this is the only way to truly fix it, and it is outside `mlrshift`'s reweight-only contract).

`mlrshift` makes this trilemma **explicit** rather than hiding it: the `shift` overflow `Note:`, `inspect`'s "no previewed donor reaches … without overflow" message, and the loud **`RTP lock: ✗ NOT held`** warning all surface the exact tradeoff being forced.

### Numerical fidelity

Weights are integers. Rounding is **half-away-from-zero**, with the **last row in each distribution absorbing the remainder**, so total mass is conserved exactly. The resulting drift is **sub-0.001%**.

> Determinism note: equally-heavy violating values are ordered deterministically (by heaviest row, then payout as a tie-break), so the output is stable across runs.
