# ProxySQL: a runtime-edition ceiling

The existing reading remains L2, with L1 good and L2 excellent. Symbol recall
is 1 and precision .9544 (356 matched of 373 emitted); all 10 reference pairs
agree. This assessment does not take a new reading or raise any grade.

`calibration/oracles/proxysql/README.md` identifies the 17 extra definitions:
variables in `src/proxysql.cfg` belonging to the MCP and GenAI feature groups,
and `debug`. The dialect intentionally reads valid declared variables even when
the measured runtime does not carry that feature. Filtering the declarations,
removing the file, or using a scanner to manufacture tier-A truth would hide
the mismatch rather than resolve it.

The pinned source remains 7ddb3dc015d1440e8a0fb0bbafa2df9aaaa2669d.
Its [configuration](https://github.com/sysown/proxysql/blob/7ddb3dc015d1440e8a0fb0bbafa2df9aaaa2669d/src/proxysql.cfg)
declares these groups. Its
[Makefile](https://github.com/sysown/proxysql/blob/7ddb3dc015d1440e8a0fb0bbafa2df9aaaa2669d/Makefile)
gates the GenAI plugin on `PROXYSQL40` and has separate debug build targets.
The [v3.0.11 release](https://github.com/sysown/proxysql/releases/tag/v3.0.11)
was inspected for an existing native oracle: its binary assets target Linux.
No release was installed, source build attempted, or oracle executed here.
The source does contain Darwin handling; lack of a release asset does not
prove that a source build is impossible.

The new `LB-Q-proxysql-rest` records the present ceiling and the retry trigger:
a source-matched runtime with these feature groups and debug setting attested
through its database, followed by the unchanged corpus and recipe. The current
dialect, required kinds, corpus, oracle, and committed reading are preserved.
No shared strategy or extractor epoch changes. No decision model applies:
this is a deterministic evidence mismatch, not a judgment site.
