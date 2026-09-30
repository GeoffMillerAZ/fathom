with Ada.Numerics;

package body Shapes is

   Pi : constant Float := Ada.Numerics.Pi;

   procedure Check (S : Shape) is
   begin
      if Area (S) < 0.0 then
         raise Invalid_Shape;
      end if;
   end Check;

   procedure Move (S : in out Shape; By : Point) is
   begin
      S.Origin.X := S.Origin.X + By.X;
      S.Origin.Y := S.Origin.Y + By.Y;
      Check (S);
   end Move;

   function "+" (L, R : Point) return Point is
   begin
      return (X => L.X + R.X, Y => L.Y + R.Y);
   end "+";

   function Area (S : Shape) return Float is
      function Sq (V : Float) return Float is
      begin
         return V * V;
      end Sq;
   begin
      case S.Kind is
         when Circle   => return Pi * Sq (Unit_Scale);
         when Square   => return Sq (Unit_Scale);
         when Triangle => return 0.5 * Sq (Unit_Scale);
      end case;
   end Area;

   function Scaled
     (S      : Shape;
      Factor : Float := Unit_Scale)
      return Shape
   is
      Result : Shape := S;
   begin
      Move (Result, (X => Factor, Y => Factor));
      return Result;
   end Scaled;

   package body Registry is

      Total : Natural := 0;

      procedure Register (S : Shape) is
      begin
         Shapes.Check (S);
         Total := Total + 1;
      end Register;

      function Count return Natural is
      begin
         return Total;
      end Count;

   end Registry;

end Shapes;
