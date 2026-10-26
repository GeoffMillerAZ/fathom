// Fixture: definitions an HLSL file makes.
#define MAX_LIGHTS 16
#define SQR(x) ((x) * (x))

typedef float4 Color;
typedef uint2 Cell;

struct VSInput
{
    float3 position : POSITION;
    float2 uv       : TEXCOORD0;
};

struct VSOutput
{
    float4 position : SV_Position;
    float2 uv       : TEXCOORD0;
};

cbuffer Frame : register(b0)
{
    float4x4 viewProj;
    float3   eye;
};

cbuffer Material
{
    float4 tint;
};

Texture2D<float4> gAlbedo : register(t0);
SamplerState gSampler : register(s0);
static const float kEpsilon = 1e-4;
float gScale = 2.0;

float Square(float v)
{
    return v * v;
}

float3 Shade(float3 n,
             float3 l,
             float3 c)
{
    return saturate(dot(n, l)) * c;
}

VSOutput MainVS(VSInput input)
{
    VSOutput o;
    o.position = mul(float4(input.position, 1.0), viewProj);
    o.uv = input.uv;
    return o;
}

float4 MainPS(VSOutput i) : SV_Target0
{
    float4 c = gAlbedo.Sample(gSampler, i.uv);
    return float4(Shade(c.rgb, eye, tint.rgb), Square(c.a));
}

[numthreads(8, 8, 1)]
void BlurCS(uint3 id : SV_DispatchThreadID)
{
    float v = Square(1.0);
}
