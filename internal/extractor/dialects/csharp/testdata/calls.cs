using System;
using System.Linq;

namespace Acme.Calls;

public class Worker<T>
{
    public void Plain() => Helper(1);

    public void Generic() => Make<int>(2);

    public void Member(Registry reg, Worker<int> w)
    {
        reg.Add(1);
        w.Convert<string>(3);
        Registry.Create<Worker<int>>(4);
        Registry.Pair<System.Exception, string>(5);
    }

    public void Names(string arg) => Console.WriteLine(nameof(arg));

    public void Conditional(Registry reg) => reg?.Touch<int>();

    public void SelfCalls()
    {
        this.Plain();
        base.ToString();
    }

    public void Delegates(Action<int> callback, Func<int, int> map)
    {
        callback(1);
        map(2);
        var items = new[] { 1, 2 }.Where(x => x > 0).Select(x => x * 2);
        var made = new Registry();
        var size = typeof(Registry).Name + default(int);
    }

    private static void Helper(int n) { }

    private static T Make<T>(int n) => default!;

    private T Convert<T>(int n) => default!;
}

public class Registry
{
    public void Add(int n) { }

    public void Touch<T>() { }

    public static T Create<T>(int n) => default!;

    public static void Pair<A, B>(int n) { }
}
