-- A comment may open a body; a bracketed key names nothing.
local M = {}

function M.commented(a)
  -- explains the next line
  return a
end

function M.trailing(b) -- a note on the header line
  return b
end

function M.only_comment() -- nothing else in here
end

M.assigned = function(c) --[[ block comment ]] return c end

local table_of = {
  first = function(x)
    -- a leading comment
    return x
  end,
  [KEY] = function(y) return y end,
  ["literal"] = function() end,
  [1] = function() end,
}

M["indexed"] = function(v) return v end

return M
