# W8-2a library builder: work split and dispatchability

Source base: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`. Map only, published before draft authoring.
All 14 rows in docs/plans/library-builder-kit.md remain open. Own drafts only
`docs/contracts/library-kit*.md`. No implementation/measurement/door budget change.

| Split | Rows | Exact implementation boundary | Present entry state |
|---|---|---|---|
| Foundation | LBK-2, LBK-3 | two row readings; docs/use-cases/library-builder-contract.md | First. REC-1 exists; LIB-11 checked. Verify landed parents independently. Requirements five fields may stay unknown (LK-D1/D4). |
| Grader | LBK-6 | pkg/lang/langtest/cases{,_test}.go, pkg/lang/testdata/api/langtest.txt; row reading | After foundation and actual HZ-16/LIB-10 receipts. CheckPack remains unchanged (LK-D6). |
| Consumers/docs | LBK-4,5,7,8,11 | exact row-owned docs, examples/lang/{terraformhazard,golifetime,cuepolicy}/ row files | Separate leaf ownership; LIB-7/HZ-10, RL-2/RL-9/CHG-10, HZ-13..18/OQ3 are not supplied by LBK. Cookbook last. |
| Evidence | LBK-1,9,10,12 | each row's single docs/bench/readings/library-builder-kit/lbk-n-*.md | LBK-1 requires REC-1/HZ-0. Baseline then frozen independent reread; trial after cookbook/release. Raw inputs missing means blocked. |
| Present family | LBK-13 | pkg/lang/family_test.go, pkg/lang/testdata/family/**; row reading | After foundation; existing closure/no-values proofs are reusable. No moves/renames. |
| Later family | LBK-14 | pkg/lang/family_records_bench_test.go, pkg/lang/testdata/family/records-bench/**; row reading | After LBK-13 + actual REC-6/BEN-1. No speculative leaf/driver names. |

LK-D6/7 and row needs settle these boundaries. Only explicit fixture trees are
permitted additions. Current implementation entry is not established by this map:
no RED run, no package run, no native census, no gates. W3-2 real RED belongs to
implementer. W2-1 door 80/consumer 81 includes root and has zero headroom.

Independent author allocation is possible for evidence/present-family drafts and
consumer fixture manifests after the foundation handoff. This author retains all
rows unless foreman explicitly reallocates a split; the map grants no reservation.
Grader and foundation share callback decisions; consumers consume their exact
commits. No additional author or implementation is dispatched here.
