--!strict
-- Typed functions: parameter types, return types, generics, methods, nesting.
local M = {}

local function add(a: number, b: number): number
  return a + b
end

function M.sub(self: Point, n: number): Point
  return self
end

function M:method(x: string)
end

function M.generic<T>(value: T): T
  local function inner(v: T): T
    return v
  end
  return inner(value)
end

local assigned = function(x: number): number return x end

M.handler = function(
  player: Player,
  count: number
): ()
  print(player, count)
end

local table_of = {
  run = function(self: any) end,
  ["on-close"] = function() end,
  [KEY] = function() end,
  value = 1,
}

return M
