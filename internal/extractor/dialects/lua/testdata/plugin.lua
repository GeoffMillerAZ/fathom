-- A gateway plugin handler: a module-level handler table, local helpers, phase methods.
local kong = kong
local ngx = ngx

local _M = {}

local PluginHandler = {
  PRIORITY = 1000,
  VERSION = "1.0.0",
}

local function build_key(conf, consumer)
  return conf.prefix .. ":" .. consumer.id
end

function _M.check(key, limit)
  local ok, err = pcall(function()
    kong.cache:get(key, nil, fetch, key)
  end)
  return ok and limit or err
end

function PluginHandler:access(conf)
  local key = build_key(conf, kong.client.get_consumer())
  if not _M.check(key, conf.limit) then
    return kong.response.exit(429, { message = "limit reached" })
  end
end

PluginHandler.header_filter = function(self, conf)
  kong.response.set_header("X-Plugin", "on")
end

function PluginHandler:log(conf)
  ngx.log(ngx.INFO, "done")
end

return PluginHandler
