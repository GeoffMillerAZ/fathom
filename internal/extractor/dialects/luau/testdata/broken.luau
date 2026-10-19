-- A syntax error the parser must survive: the definitions around it are still found.
local function before(a: number): number
  return a
end

local function broken(
  return

local function after(b: string): string
  return b
end

type Alias = { x: number }
