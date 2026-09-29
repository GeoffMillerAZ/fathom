-- Call shapes, and which definition owns each call.
local util = require("util")
util.setup()

local function helper() end

local function plain()
  helper()
  helper()
end

local function qualified()
  util.trim(" x ")
  string.format("%d", 1)
  vim.api.nvim_buf_set_lines(0, 0, -1, false, {})
end

local function methods(obj)
  obj:run()
  self:step()
  obj.child:stop()
end

local function sugar()
  log "message"
  configure { verbose = true }
  obj:say "hi"
end

local function chained()
  builder():name("x"):build()
  local s = ("x"):rep(3)
  return s
end

-- not calls the query can name: an indexed callee and a callee that is itself a call
local function unnamed(t, f)
  t[1]()
  f()()
end

-- a call inside a closure belongs to the named definition around it
local function closures(items)
  pcall(function()
    risky(items)
  end)
  for _, item in ipairs(items) do
    visit(item)
  end
end

-- a call inside a nested named definition belongs to that definition, not its parent
local function outer()
  local function inner()
    deep_call()
  end
  outer_call()
  return inner
end

-- a call inside a function stored in a table belongs to the field's function
local dispatch = {
  open = function(path)
    return io.open(path)
  end,
  close = function(handle)
    handle:close()
  end,
}

-- the same callee twice, in different shapes, is one call
local function repeated(a)
  a.go()
  a:go()
  go()
end
