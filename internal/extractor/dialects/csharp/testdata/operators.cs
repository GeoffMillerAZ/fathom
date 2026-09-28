namespace MyApp.Ops;

public class Money
{
    public int Cents;

    public static explicit operator bool(Money m) => m.Cents != 0;

    public static implicit operator Money(int cents) => new Money();
}
