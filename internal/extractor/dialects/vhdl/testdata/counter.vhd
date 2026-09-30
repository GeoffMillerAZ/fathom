-- Counter: an entity, its architecture, processes and a component instance.
library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;
use work.counter_pkg.all;

entity counter is
  generic (
    WIDTH : positive := 8
  );
  port (
    clk   : in  std_logic;
    rst   : in  std_logic;
    count : out unsigned(WIDTH - 1 downto 0)
  );
end entity counter;

architecture rtl of counter is
  signal value : unsigned(WIDTH - 1 downto 0) := (others => '0');
  constant LIMIT : natural := 2 ** WIDTH - 1;

  component debounce is
    port (clk : in std_logic; d : in std_logic; q : out std_logic);
  end component debounce;
begin

  tick : process (clk, rst)
  begin
    if rst = '1' then
      value <= (others => '0');
    elsif rising_edge(clk) then
      value <= next_count(value, LIMIT);
    end if;
  end process tick;

  process (clk)
  begin
    if rising_edge(clk) then
      report "clock";
    end if;
  end process;

  u_deb : debounce
    port map (clk => clk, d => rst, q => open);

  count <= value;
end architecture rtl;
