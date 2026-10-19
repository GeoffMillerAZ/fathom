-- lua_searchpath.lua — reads module names on stdin, prints "name<TAB>file" for each that Lua's own
-- package.searchpath finds under the path given as the first argument, run from the subject root. A file outside
-- the subject (a ".." path) is not printed. The leading "./" is dropped.
local template = arg[1]
for name in io.lines() do
  local file = package.searchpath(name, template)
  if file and not file:find("^%.%./") and not file:find("/%.%./") then
    io.write(name, "\t", (file:gsub("^%./", "")), "\n")
  end
end
