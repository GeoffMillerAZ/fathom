// Calls: plain, intrinsic, method on a resource, nested, constructor-shaped and
// attribute-shaped (neither is a call), and a macro invocation.
#define LOAD(b, i) b.Load(i)

Texture2D<float4> gTex : register(t0);
RWStructuredBuffer<uint> gOut : register(u0);
SamplerState gSamp : register(s0);

uint Hash(uint x)
{
    return x * 747796405u + 2891336453u;
}

float4 Fetch(float2 uv, uint i)
{
    float4 c = gTex.Sample(gSamp, uv);
    gOut[i] = Hash(asuint(c.x));
    return float4(c.rgb, 1.0) * lerp(0.0, 1.0, Hash(i));
}

[numthreads(64, 1, 1)]
void MainCS(uint3 id : SV_DispatchThreadID)
{
    float4 v = Fetch(float2(id.xy), id.x);
    InterlockedAdd(gOut[0], 1);
    gOut[id.x] = (uint)v.x + LOAD(gOut, 0);
}

#define RS "RootFlags(0)"

[RootSignature(RS)]
[NumThreads(8, 8, 1)]
void UpperCS(uint3 id : SV_DispatchThreadID)
{
    gOut[id.x] = Hash(id.y);
}

struct Ray
{
    float3 origin;
    static Ray create(float3 o) { Ray r; r.origin = o; return r; }
};

float3 Origin()
{
    return Ray::create(float3(0, 0, 0)).origin;
}

// User functions that share an attribute's word stay calls when a body calls
// them; the attributes ([domain], [instance], [shader], [unroll(4)]) and the
// register and packoffset clauses are excluded by where they are written.
float4 Instance(float4 c) { return c; }
float4 Domain(float4 c) { return c; }
float4 Shader(float4 c) { return c; }

cbuffer Params : register(b1)
{
    float4 gScale : packoffset(c0);
};

[domain("tri")]
[instance(2)]
float4 Main(float4 c : COLOR) : SV_Target
{
    [unroll(4)] for (int i = 0; i < 4; i++) { c += gScale; }
    return Instance(c) + Domain(c) + Shader(c);
}
