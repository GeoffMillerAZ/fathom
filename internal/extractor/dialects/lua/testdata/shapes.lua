-- Every way Lua spells a function definition.
local M = {}

-- a global function
function hello(name)
  print(name)
end

-- a local function
local function helper(a, b)
  return a + b
end

-- a dotted module function whose signature spans three lines
function M.add(a,
               b,
               c)
  return helper(a, b) + c
end

-- a deeply dotted function with an empty body
function M.sub.deep()
end

-- a colon method
function M:method(x)
  return x
end

-- assigned functions: dotted, local, global, multi-line signature, empty body
M.assigned = function(a)
  return a
end

local anon = function() end
someglobal = function() return 1 end

M.wide = function(left,
                  right)
  return left .. right
end

-- functions as table constructor fields
local handlers = {
  on_open = function(ev) return ev end,
  on_close = function(ev)
    return ev
  end,
  named = helper,
  nested = { deeper = function() end },
  ["quoted"] = function() end,
}

-- nesting: a function inside a function inside a method
function M:outer()
  local function inner(z)
    local deepest = function() return z end
    return deepest()
  end
  return inner(1)
end

-- none of these is a definition
local value = 42
local alias = helper
M.constant = "x"
local meta = setmetatable({}, { __index = M })

return M
