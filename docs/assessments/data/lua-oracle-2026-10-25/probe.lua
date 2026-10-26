local M = {}
local function helper(x) return x + 1 end
function M.run(x) return helper(x) end
local function unused(x) return helper(x) end
return M
