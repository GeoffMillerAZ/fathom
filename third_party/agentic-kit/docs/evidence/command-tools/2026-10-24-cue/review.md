# Independent source review

Four source files were reviewed after reproducing both CUE 0.17.1 failures.
No confirmed blocker remained. Byte ceilings match Go, and the explicit closed
control union preserves mandatory full sampling, hold, no maintenance and no tuners.
Advisory ordering and unknown-field/class rejection remain. New tests compare actual
Go/CUE control acceptance and concrete ASCII/multibyte byte boundaries.

The reviewer independently ran the formerly failing fixtures with CUE 0.16.1 and
0.17.1 and rejected a mandatory control with an unknown field. Controller full gate,
helper snapshot and remote CI are separately scoped instruments. A skipped CUE test
is not qualification. No gate, workflow, library API or dependency change was made.
