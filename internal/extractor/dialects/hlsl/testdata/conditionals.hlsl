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
