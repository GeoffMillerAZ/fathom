--  Workers: task and protected declarations.
package Workers is

   Queue_Size : constant Positive := 8;

   protected type Counter is
      procedure Increment;
      function Value return Natural;
      entry Wait_For (Target : Natural);
   private
      Count : Natural := 0;
   end Counter;

   task type Worker (Id : Positive) is
      entry Start (Job : Natural);
      entry Stop;
   end Worker;

   task Logger is
      entry Log (Message : String);
   end Logger;

   protected Registry is
      procedure Add (Id : Positive);
   end Registry;

   type Worker_Access is access Worker;

   procedure Run_All;

end Workers;
