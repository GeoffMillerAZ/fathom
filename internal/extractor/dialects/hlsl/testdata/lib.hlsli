// Shared header: types and helpers an including shader uses.
#ifndef LIB_HLSLI
#define LIB_HLSLI

#define PI 3.14159265

struct Light
{
    float3 color;
    float  range;
};

typedef Light LightList[4];

cbuffer LightParams : register(b1)
{
    Light gLights[4];
};

float Attenuate(float d, float range)
{
    return saturate(1.0 - d / range);
}

float3 Radiance(Light l, float d)
{
    return l.color * Attenuate(d, l.range);
}

#endif
