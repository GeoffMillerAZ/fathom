// Fixture: a struct's methods with a body, one defined out of line, and their
// calls; a prototype in the struct is no definition.
struct Light
{
    float3 color;
    float intensity;

    static Light make(float3 c) { Light l; l.color = c; l.intensity = 1; return l; }

    float3 radiance(float d)
    {
        return color * intensity * rcp(max(d * d, 1e-4));
    }

    float luminance();
};

float Light::luminance()
{
    return dot(color, float3(0.2126, 0.7152, 0.0722));
}

float3 Shade(float3 c, float d)
{
    Light l = Light::make(c);
    return l.radiance(d) * l.luminance();
}
