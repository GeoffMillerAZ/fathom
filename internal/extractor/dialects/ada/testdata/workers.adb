with Ada.Text_IO; use Ada.Text_IO;

package body Workers is

   protected body Counter is

      procedure Increment is
      begin
         Count := Count + 1;
      end Increment;

      function Value return Natural is
      begin
         return Count;
      end Value;

      entry Wait_For (Target : Natural) when Count >= Target is
      begin
         null;
      end Wait_For;

   end Counter;

   protected body Registry is
      procedure Add (Id : Positive) is
      begin
         null;
      end Add;
   end Registry;

   task body Worker is
      Current : Natural := 0;
   begin
      accept Start (Job : Natural) do
         Current := Job;
      end Start;
      Logger.Log ("started");
      accept Stop;
   end Worker;

   task body Logger is
   begin
      loop
         select
            accept Log (Message : String) do
               Put_Line (Message);
            end Log;
         or
            terminate;
         end select;
      end loop;
   end Logger;

   procedure Run_All is
      W : Worker (Id => 1);
   begin
      W.Start (Job => 10);
      W.Stop;
      Registry.Add (1);
   end Run_All;

end Workers;
