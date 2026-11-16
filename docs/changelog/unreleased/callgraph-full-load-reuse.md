### Changed

- Full call-graph loads reuse matching, unpatched syntax graphs and trim owned adjacency slices without extra copies. A five-trial [reading](../../bench/readings/realignment/eng-load-cost-p1-p2.md) improved the median from 1.497 s to 1.250 s. The one-second target remains unmet.
