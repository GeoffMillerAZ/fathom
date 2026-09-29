using Assert = MyApp.Testing.XUnitAssert;
using Scenario = MyApp.Testing.Fixtures.PolicyExtensions.Scenario;
using static MyApp.Tools.Hammer;
using Fact = Xunit.FactAttribute;
using Tools = MyApp.Tools;
using Nail = Vendor.Tools.Hammer;

namespace MyApp.Tests
{
    public class AliasTests
    {
        public void Check()
        {
            Assert.Equal();
            Hit();
            Tools.Hammer.Hit();
            Nail.Hit();
        }
    }
}
