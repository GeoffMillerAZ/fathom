# Jenkins load edges' ingest cost (wave lf8, item cifmt; K26, K27, K30)

**Verdict: within the cost gate. The one ingest-output change of this item is the Jenkins pass's literal
`load` edge (LB-F560); GitHub Actions and Helm changed no dialect, only their oracles. On the subject
where the output changes (ROCm/MIOpen, 1 load edge) cold first ingest reads +10.3% wall (0.48 s to
0.53 s, 50 ms) and +4.7% CPU, warm +3.2% wall, store equal; on HariSekhon/Jenkins (vars/ step calls
only, no load: the output is identical) and on fathom itself (no Jenkins pipeline: identical) nothing
regresses (cold and warm wall and CPU equal or lower, store equal). No change exceeds +20% store or
+15% wall, so no owner ruling is owed (K26). K27: the base did the same pass (shell, build and vars/
step edges); the load scan is new work on the same files, measured here, not skipped.**

Measured 2026-10-15, base `fathom` built at 99aa205a (origin/main when the wave started), branch binary
at 9576662a (the Jenkins load edge and its oracle), same machine, each ingest a fresh process into a
fresh scratch store (cold, K30) then again into the same store (warm). Other wave workers ran beside
it (load average 8 to 26), so wall is noisy; the medians of interleaved rounds (the order alternating
each round) and the child CPU time are the reading.

## Medians

| Subject | Rounds | Cold wall | Cold CPU | Warm wall | Warm CPU | Store bytes |
|---|---|---|---|---|---|---|
| ROCm/MIOpen 06977176 (Jenkinsfile, vars/) | 5 | 0.48 s to 0.53 s (+10.3%) | 0.52 s to 0.54 s (+4.7%) | 0.11 s to 0.11 s (+3.2%) | 0.12 s to 0.12 s (+4.8%) | 1,441,792 equal |
| HariSekhon/Jenkins 99c84dc2 | 5 | 1.37 s to 1.22 s (-11.0%) | 1.53 s to 1.42 s (-7.1%) | 0.08 s to 0.08 s (-3.6%) | 0.11 s to 0.10 s (-6.2%) | 1,171,456 equal |
| fathom at 99aa205a | 3 cold, 7 warm | 225.57 s to 77.30 s (-65.7%) | not read | 1.94 s to 1.79 s (-7.7%) | 2.59 s to 2.47 s (-4.8%) | 171,511,808 equal |

What the output adds: MIOpen's store gains one typed edge (0 to 1, `load "vars/utils.groovy"`), every
other table's row count equal; HariSekhon's and fathom's stores hold the same row count in every table
at base and head. The store files are equal in bytes because one edge fits the pages already allocated.

## Noise, disclosed

fathom's cold rounds ran while the machine's load average reached 26 (base 225.57 s, 254.23 s, 76.86 s;
head 77.30 s, 131.16 s, 65.56 s): the cold medians say only that the branch is not slower, never a
speed-up. A first warm reading on fathom, medians of 3 rounds taken in the same window, read +27.8% wall
(1.82 s to 2.33 s; head 2.33, 3.17 and 2.07 against base 2.06, 1.80 and 1.82); re-read over 7
interleaved rounds with CPU time it is -7.7% wall and -4.8% CPU, so the first figure was the load, not
the change. The
MIOpen and HariSekhon base rounds include one outlier each (17.55 s and 3.11 s cold), which the medians
leave out.

Method: medians of 3 interleaved rounds (fathom cold), 5 (the Jenkins subjects) and 7 (fathom warm);
wall by a monotonic clock around each process, CPU as the child's user plus system time, store as the
SQLite file and its sidecars.
