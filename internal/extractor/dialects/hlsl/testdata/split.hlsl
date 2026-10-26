// Fixture: a statement head split across #if/#else, which the grammar reads
// in error as written; the pre-parse step's conditional-blanked candidate (c)
// reads it as two nested heads, so the file parses clean.
float4 Pick(float4 c, float t)
{
#if USE_LUMA
    if (dot(c.rgb, float3(0.3, 0.6, 0.1)) > t)
#else
    if (c.a > t)
#endif
    {
        c = saturate(c);
    }
    return c;
}

float Sum(uint n)
{
    float s = 0;
#ifdef UNROLLED
    for (uint i = 0; i < 4; i++)
#else
    for (uint i = 0; i < n; i++)
#endif
    {
        s += Weight(i);
    }
    return s;
}
