--  A generic child package: formal parameters, then the package it declares.
generic
   type Element is private;
   with function "=" (L, R : Element) return Boolean is <>;
package Containers.Generic_Stack is

   type Stack is limited private;

   procedure Push (S : in out Stack; Item : Element);
   function Pop (S : in out Stack) return Element;
   function Is_Empty (S : Stack) return Boolean;

   Stack_Empty : exception;

private

   type Node;
   type Node_Access is access Node;

   type Stack is limited record
      Top : Node_Access;
   end record;

end Containers.Generic_Stack;
