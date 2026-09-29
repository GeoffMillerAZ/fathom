-- One statement is malformed; the definitions around it are still found.
local function before(a)
  return a
end

local broken = = 1

function after(b)
  return before(b)
end
