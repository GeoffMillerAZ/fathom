// The fixture's header: a sampler, a material type and a helper the unit uses.
SamplerState linearSampler : register(s0);

struct Material
{
    float rough;
};

float helper(float x)
{
    return saturate(x);
}
