// Fixture: constant and texture buffers whose headers carry a register clause
// (read through the hlsl-macros pre-parse step).
cbuffer Scene : register(b1, space2)
{
    float4x4 world;
    float    time;
};

tbuffer Palette : register( t4 )
{
    float4 colors[8];
}

Texture2D gAlbedo : register(t0, space1);

float4 shade(float2 uv)
{
    return gAlbedo.Load(int3(uv, 0)) * colors[0] * time;
}
