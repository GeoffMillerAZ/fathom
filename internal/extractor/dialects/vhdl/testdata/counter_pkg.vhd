-- Counter_pkg: a package with types, constants and subprograms, and its body.
library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

package counter_pkg is

  constant DEFAULT_WIDTH : positive := 8;

  type state_t is (idle, counting, done);

  subtype byte_t is unsigned(7 downto 0);

  type frame_t is record
    header  : byte_t;
    payload : unsigned(31 downto 0);
    valid   : std_logic;
  end record frame_t;

  type frame_array_t is array (natural range <>) of frame_t;

  function next_count(v : unsigned; limit : natural) return unsigned;

  function is_full(
    v     : unsigned;
    limit : natural
  ) return boolean;

  procedure clear(signal v : out unsigned);

end package counter_pkg;

package body counter_pkg is

  function next_count(v : unsigned; limit : natural) return unsigned is
  begin
    if is_full(v, limit) then
      return to_unsigned(0, v'length);
    end if;
    return v + 1;
  end function next_count;

  function is_full(
    v     : unsigned;
    limit : natural
  ) return boolean is
  begin
    return to_integer(v) = limit;
  end function is_full;

  procedure clear(signal v : out unsigned) is
    function zero return unsigned is
    begin
      return to_unsigned(0, v'length);
    end function zero;
  begin
    v <= zero;
  end procedure clear;

end package body counter_pkg;
