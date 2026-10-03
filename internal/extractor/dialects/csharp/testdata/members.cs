namespace Shop.Orders
{
    public delegate void OrderHandler(Order order);

    public enum Status { Draft, Paid = 2, Shipped = Paid + 1 }

    public class Order
    {
        private static readonly Registry Shared = Registry.Create("orders");
        private int count = 0, limit;
        public event OrderHandler Placed;
        public event OrderHandler Cancelled
        {
            add { Log.Write("add"); }
            remove { Log.Write("remove"); }
        }

        public decimal this[int line] => Lines.Price(line);

        public Total Sum => new Total(count);

        public Order()
        {
            var items = new List<Item>();
            var box = new Shop.Box { Size = 2 };
            var any = new global::Shop.Box();
            var none = new int[3];
            Order self = new(this);
        }

        ~Order() { Log.Write("gone"); }
    }
}
