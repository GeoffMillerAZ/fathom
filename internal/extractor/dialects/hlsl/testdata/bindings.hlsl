// Fixture: binding macros that expand to resource declarations, written at
// file scope with no semicolon (read through the hlsl-macros pre-parse step).
#include "lib.hlsli"

DECLARE_FRAME_BINDINGS(0, 1, 2)
DECLARE_SHADOW_BINDINGS(3)

RWTexture2D<float4> output;

[numthreads(8, 8, 1)]
void main(uint2 px : SV_DispatchThreadID)
{
    output[px] = shadow_term(px);
}
