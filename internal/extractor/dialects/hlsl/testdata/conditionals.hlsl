// Fixture: file-scope declarations under preprocessor conditionals, locals
// under one inside a function, and a Vulkan-bound cbuffer.
#if USE_SHADOWS
Texture2D shadowMap : register(t1);
#else
Texture2D fallbackMap : register(t1);
#endif

#ifdef USE_FOG
#if FOG_HEIGHT
float fogHeight;
#endif
float fogDensity;
#endif

[[vk::binding(5)]] cbuffer _ {
    float4 outputSize;
};

[[vk::binding(6)]] cbuffer LightData {
    float4 lightColor;
};

float4 Sample(uint index)
{
#ifdef USE_POINTERS
    uint address = index * 8;
    float scale = 2.0;
#endif
    return shadowMap.Load(int3(index, 0, 0));
}

// An #elif chain to its #else, and an #else under a nested #if: every branch
// holds a file-scope variable.
#if QUALITY == 1
float qualityLow;
#elif QUALITY == 2
float qualityMid;
#else
float qualityHigh;
#endif

#ifdef USE_A
#if USE_B
float bothAB;
#else
float onlyA;
#endif
#endif

float4 Shade(float4 c)
{
#if USE_A
#if USE_B
    float nestedLocal = 1.0;
#else
    float otherLocal = 2.0;
#endif
#endif
    return c * nestedLocal;
}
