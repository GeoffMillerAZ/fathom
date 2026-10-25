// A file with a syntax error in the middle: the definitions around it are still extracted.
float Good(float a)
{
    return a * 2.0;
}

float Broken(float a, @#$)
{
    return a +;
}

struct Survivor
{
    float x;
};

float After(float b)
{
    return Good(b);
}
