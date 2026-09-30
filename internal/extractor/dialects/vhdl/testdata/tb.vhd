-- Tb: a testbench entity with no ports, a process with waits, and a subprogram in a process.
library ieee;
use ieee.std_logic_1164.all;

entity tb is
end entity tb;

architecture bench of tb is
  signal clk : std_logic := '0';
  signal rst : std_logic := '1';
begin

  clk <= not clk after 5 ns;

  stimulus : process
    procedure pulse(signal s : out std_logic) is
    begin
      s <= '1';
      wait for 10 ns;
      s <= '0';
    end procedure pulse;

    function is_high(s : std_logic) return boolean is
    begin
      return s = '1';
    end function is_high;
  begin
    pulse(rst);
    wait until rising_edge(clk);
    assert is_high(clk) report "clock is low" severity note;
    wait;
  end process stimulus;

  dut : entity work.counter
    port map (clk => clk, rst => rst, count => open);

end architecture bench;
