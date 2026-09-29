namespace Fixture.Branches
{
    public class Reader
    {
        public int Read(int a, int b)
        {
            return Sum(
#if FEATURE
                a,
#endif
                b);
        }

#if FEATURE
        public void Fast() {}
#else
        public void Slow() {}
#endif
    }
}
