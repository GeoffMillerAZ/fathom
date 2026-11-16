### Changed

- Full graph loads reuse matching, unpatched syntax graphs and trim owned adjacency slices without extra copies. In a five-trial [reading](https://github.com/GeoffMillerAZ/fathom/blob/main/docs/bench/readings/realignment/eng-load-cost-p1-p2.md), the snapshot+syntax+BOTH median improved from 1.497 s to 1.250 s. The one-second target remains unmet.
