--  One specification is malformed; the parser closes Broken at `end Oops;`, and the function after it is a unit of its own.
procedure Broken is

   procedure Before (X : Integer) is
   begin
      null;
   end Before;

   procedure Oops (Y : Integer is
   begin
      null;
   end Oops;

   function After (Z : Integer) return Integer is
   begin
      return Before_Value (Z);
   end After;

begin
   Before (1);
end Broken;
