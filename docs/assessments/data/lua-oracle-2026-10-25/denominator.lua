-- Resolve every independently enumerated literal through the subject runtime's loaders.
local root = assert(vim.env.LUA_TRACE_SUBJECT)
vim.cmd('source scripts/minimal_init.vim')
local f = assert(io.open(vim.env.LUA_TRACE_SITES))
local sites = vim.json.decode(f:read('*a')); f:close()
local population = {}
for _, site in ipairs(sites.stmts) do
  for _, searcher in ipairs(package.loaders) do
    local loader = searcher(site.module)
    if type(loader) == 'function' then
      local info = debug.getinfo(loader, 'S')
      if info.source:sub(1,1) == '@' then
        local target = vim.fn.fnamemodify(info.source:sub(2), ':p'):gsub('/%./','/')
        if target:sub(1,#root+1) == root .. '/' then
          site.to_path = target:sub(#root+2); table.insert(population, site)
        end
      end
      break
    end
  end
end
local out = assert(io.open(vim.env.LUA_TRACE_POPULATION, 'w'))
out:write(vim.json.encode({counts=sites.counts, imports=population})); out:close()
vim.cmd('qa!')
