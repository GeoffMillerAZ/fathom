## Excellence board (generated)

Generated from the queue's `tier` field (T1..T6; an entry without one is outside the
excellence target set), the committed readings and the thresholds of design §1.2; never
hand-edit it. Level counts are the level met exactly; `none` is measured with no level met;
`refused`, `tolerated` and `unsupported` claim no level. At target is a level met at or above the
item's target; the last column also needs every level up to the target graded excellent.

| Tier | Kind | Items | L1 | L2 | L3 | L4 | None | Refused | Tolerated | Unsupported | At target | At target, all excellent |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | language | 9 | 3 | 1 | 1 | 1 | 1 | 0 | 1 | 1 | 4 | 2 |
| T2 | tool format | 3 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 1 |
| T3 | tool format | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| T4 | language | 2 | 0 | 0 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 |
| **Total** | language | 11 | 3 | 1 | 1 | 1 | 1 | 1 | 2 | 1 | 4 | 2 |
| **Total** | tool format | 4 | 0 | 2 | 0 | 0 | 0 | 0 | 1 | 1 | 1 | 1 |

### T1

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Go (.go) | language | L4 | tolerated | no reading (LB-Q-go) |
| Swift | language | L3 | unsupported | no dialect (LB-Q-swift) |
| Python | language | L4 | L1 (L1 excellent) | L2 not met: call_recall not measured |
| Alpha | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |
| Beta | language | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: file_edge_recall 0.4 < 0.5 |
| Gamma | language | L2 | L2 (L1 excellent, L2 good) | L2 good: oracle tier C caps at good |
| Delta | language | L3 | L1 (L1 excellent) | L2 not met: call_recall 0.6 < 0.75 |
| Epsilon | language | L2 | none (measured) | L1 not met: sym_recall 0.5 < 0.85 |
| Zeta | language | L1 | L1 (L1 excellent) | excellent at target |

### T2

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Format A | tool format | L2 | L2 (L1 excellent, L2 excellent) | excellent at target |
| Format B | tool format | L3 | L2 (L1 excellent, L2 excellent) | L3 not met: xref_precision not measured |
| Format C | tool format | L2 | unsupported | no dialect (LB-Q-fmt-c) |

### T3

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Helm \| charts | tool format | L3 | tolerated | no reading (LB-Q-helm) |

### T4

| Item | Kind | Target | Level met | Next gap |
|---|---|---|---|---|
| Eta | language | L3 | refused | re-plan (LB-Q-eta-rest refused) |
| Theta | language | L2 | tolerated | no reading (LB-Q-theta) |

Untiered entries (outside the excellence target set): 1 (loose).
