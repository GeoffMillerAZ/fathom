# dxc-ast parser fixture

`fixture.hlsl` and `fixture.hlsli` are written for this test (no calibration subject's code). `fixture.dump.txt` is
the pinned DXC's text AST of the unit, unedited:

    cd calibration/oracles/dxc-ast/testdata && dxc -T lib_6_3 -HV 2021 -ast-dump ./fixture.hlsl > fixture.dump.txt

with DXC v1.9.2602 (`libdxcompiler.so: 1.9(5184-21d28f72)(1.9.0.5184)`, the release asset named in `../README.md`).
`TestParseDumpFixture` reads it.
