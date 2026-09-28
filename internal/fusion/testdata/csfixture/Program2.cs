using MyApp.Util;
using MyApp.Tools;

namespace MyApp.Consumer
{
    public class Worker
    {
        public void Go()
        {
            Helper.Assist();
            Hammer.Hit();
        }
    }
}
