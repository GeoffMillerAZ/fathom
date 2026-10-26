// A fixture written for dxc-ast's parser test: each definition kind the oracle
// writes, a struct's methods (one defined out of line), member and static
// calls, a cbuffer, and a call a macro body spells.
#include "fixture.hlsli"

#define SCALE(x) frac(x)

cbuffer Frame : register(b0)
{
    float4 tint;
};

typedef uint Index;

struct Ray
{
    float3 origin;
    static Ray create(float3 o) { Ray r; r.origin = o; return r; }
    float3 at(float t) { return origin * saturate(t); }
    float len();
};

float Ray::len() { return length(origin); }

float4 main(float4 c : COLOR) : SV_Target
{
    Ray r = Ray::create(c.xyz);
    Material m;
    m.rough = c.w;
    float s = helper(r.len() * m.rough) + SCALE(c.x);
    return float4(r.at(s), 1) * tint;
}
