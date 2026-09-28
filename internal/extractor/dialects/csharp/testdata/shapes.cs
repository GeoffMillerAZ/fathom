namespace MyApp.Shapes;

public partial class Circle : IShape
{
    public double Radius { get; set; }

    public double Diameter => Radius * 2;

    public Circle(double radius)
    {
        Radius = radius;
    }

    public double Area(
        double scale,
        bool round)
    {
        var r = Adjust(scale);
        Console.WriteLine(r);
        return r;
    }

    private double Adjust(double scale) => Radius * scale;

    public class Inner
    {
        public void Act() {}
    }
}

public partial class Circle
{
    public void Reset() {}
}

public interface IShape
{
    double Area(double scale, bool round);
}

public struct Point2D
{
    public int X;
    public int Y;
}

public enum Quadrant { NE, NW, SE, SW }

public record Vector(double X, double Y);
