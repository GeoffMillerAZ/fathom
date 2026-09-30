-- Alu: one entity with two architectures, a generate loop and direct entity instances.
library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity alu is
  port (
    a, b   : in  unsigned(7 downto 0);
    op     : in  std_logic_vector(1 downto 0);
    result : out unsigned(7 downto 0)
  );
end entity alu;

architecture rtl of alu is
begin

  select_op : process (a, b, op)
  begin
    case op is
      when "00"   => result <= a + b;
      when "01"   => result <= a - b;
      when others => result <= a and b;
    end case;
  end process select_op;

end architecture rtl;

architecture sim of alu is
  signal partial : unsigned(7 downto 0);
begin

  lanes : for i in 0 to 7 generate
    lane : entity work.adder_bit
      port map (x => a(i), y => b(i), s => partial(i));
  end generate lanes;

  drive : process
  begin
    result <= partial;
    wait for 1 ns;
  end process drive;

end architecture sim;
