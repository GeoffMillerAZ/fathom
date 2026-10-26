// An including shader: quoted and angle-bracket includes, and a use of the
// header's types and functions.
#include "lib.hlsli"
#include <Common/Packing.hlsli>
#include "missing/nothing.hlsli"

#if defined(USE_FOG)
static const float kFog = 0.5;
#endif

StructuredBuffer<Light> gSceneLights : register(t2);

float4 Lit(float3 p : POSITION) : SV_Target
{
    Light l = gSceneLights[0];
    float3 c = Radiance(l, length(p)) * PI;
    return float4(c, 1.0);
}
