--  Shapes: a package specification with types, constants, exceptions and
--  subprogram declarations.
with Ada.Text_IO;

package Shapes is

   Max_Sides  : constant := 12;
   Unit_Scale : constant Float := 1.0;

   type Point is record
      X, Y : Float;
   end record;

   type Shape_Kind is (Circle, Square, Triangle);

   subtype Side_Count is Positive range 1 .. Max_Sides;

   type Shape (Kind : Shape_Kind) is tagged private;

   Invalid_Shape, Degenerate_Shape : exception;

   procedure Move (S : in out Shape; By : Point);

   function Area (S : Shape) return Float;

   function Scaled
     (S      : Shape;
      Factor : Float := Unit_Scale)
      return Shape;

   function "+" (L, R : Point) return Point;

   function Is_Empty (S : Shape) return Boolean is (Area (S) = 0.0);

   package Registry is
      procedure Register (S : Shape);
      function Count return Natural;
   end Registry;

private

   type Shape (Kind : Shape_Kind) is tagged record
      Origin : Point;
   end record;

end Shapes;
