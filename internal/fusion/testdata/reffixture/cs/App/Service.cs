using Acme.Models;

namespace Acme.App
{
    public class Service : Base
    {
        public User Owner;

        public Helper Make()
        {
            return null;
        }

        public bool IsOpen()
        {
            return Owner.Status == Status.Open;
        }
    }
}
