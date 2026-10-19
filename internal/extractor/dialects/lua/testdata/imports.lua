-- Static requires: a string literal names the module, anything computed names none.
local plain = require("pkg.plain")
local bare = require "pkg.bare"
local single = require 'pkg.single'
local long = require([[pkg.long]])
require("pkg.effect")
local ok, guarded = pcall(require, "pkg.guarded")
local chained = require("pkg.chained").field
local dynamic = require("pkg." .. name)
local variable = require(name)
local unrelated = other("pkg.other")

local function load()
  local inner = require("pkg.inner")
  return inner
end
