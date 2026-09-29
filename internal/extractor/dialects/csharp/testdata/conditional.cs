using System;

namespace Fixture.Conditional
{
    public class Utils
    {
        public static string Name(Type type)
        {
        }
    }

    public class Widget
    {
        public void Before(object expected, object actual)
        {
        }

        public void Check(string message = null, params object[] args)
        {
            {
                {
                }
                {
                }
            }
        }

#if FEATURE
        protected Widget()
#else
        protected void Setup()
#endif
        {
            {
            }
        }

#if FEATURE
        public void OnlyFeature() {}
#else
        public void OnlyElse() {}
#endif
    }

    public static class Checks
    {
        public static void Equal(string expected, string actual)
        {
        }

        public static string Normalize(string s)
        {
            try
            {
            }
            catch (Exception ex)
            {
                {
                }
                {
                    {
                    }
                }
            }
            {
            }
        }

        public static async Task<T> ThrowsAsync<T>(Func<Task> action, params string[] messages)
        {
            {
                {
                    {
                    }
                }
                throw new Exception("Unexpected message." + Environment.NewLine + "Expected one of: " + string.Join(Environment.NewLine, messages) + Environment.NewLine + "Got: " + ex.Message + Environment.NewLine + Environment.NewLine + ex);
            }
            {
            }
        }
    }
}
