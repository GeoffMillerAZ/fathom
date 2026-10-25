// The fixture's header: a sampler and a helper the unit calls.
SamplerState linearSampler : register(s0);

float helper(float x)
{
    return saturate(x);
}
