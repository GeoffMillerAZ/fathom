-- Probe only: preserve native require/loaders and the subject's own minimal init.
local root = assert(vim.env.LUA_TRACE_SUBJECT)
local out = assert(vim.env.LUA_TRACE_OUT)
local encode = vim.json.encode
local handle = assert(io.open(out .. '/' .. vim.fn.getpid() .. '.jsonl', 'a'))
local function write(record)
  handle:write(encode(record), '\n')
  handle:flush()
end
local absolute = function(path) return vim.fn.fnamemodify(path, ':p'):gsub('/%./', '/') end
local resolved = {}
for i, searcher in ipairs(package.loaders) do
  package.loaders[i] = function(name)
    local loader, extra = searcher(name)
    if type(loader) == 'function' then
      local info = debug.getinfo(loader, 'S')
      if info.source:sub(1, 1) == '@' then resolved[name] = absolute(info.source:sub(2)) end
      if type(extra) == 'string' and extra:sub(1, 1) == '/' then resolved[name] = extra end
    end
    return loader, extra
  end
end
local native_require = require
_G.require = function(name)
  local caller
  for level = 2, 8 do
    local info = debug.getinfo(level, 'Sl')
    if not info then break end
    if info.source:sub(1, 1) == '@' then
      local source = absolute(info.source:sub(2))
      if source:sub(1, #root + 1) == root .. '/' then
        info.relative = source:sub(#root + 2); caller = info; break
      end
    end
  end
  local ok, value = pcall(native_require, name)
  if caller then
    local target = resolved[name]
    write({path=caller.relative, line=caller.currentline, module=name,
           accepted=ok, to_path=target and target:sub(1, #root + 1) == root .. '/' and target:sub(#root + 2) or nil})
  end
  if not ok then error(value, 0) end
  return value
end
vim.cmd('source scripts/minimal_init.vim')
write({environment=true, package_path=package.path, runtimepath=vim.o.runtimepath})
