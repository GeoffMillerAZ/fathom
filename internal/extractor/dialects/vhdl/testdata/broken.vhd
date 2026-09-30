-- One statement is malformed; the definitions around it are still found.
library ieee;
use ieee.std_logic_1164.all;

entity gate is
  port (a, b : in std_logic; y : out std_logic);
end entity gate;

architecture rtl of gate is
begin

  compute : process (a, b)
  begin
    y <= a and ;
  end process compute;

  other : process (a)
  begin
    y <= not a;
  end process other;

end architecture rtl;
