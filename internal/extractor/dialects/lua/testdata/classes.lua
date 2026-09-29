-- Class-style Lua: metatables, __index, constructors, colon methods and metamethods.
local Animal = {}
Animal.__index = Animal

function Animal.new(name, ...)
  local self = setmetatable({}, Animal)
  self.name = name
  self:init(...)
  return self
end

function Animal:init() end

function Animal:speak()
  return self.name .. " says " .. self:sound()
end

function Animal:sound()
  return "..."
end

local Dog = setmetatable({}, { __index = Animal })
Dog.__index = Dog

function Dog.new(name)
  local self = Animal.new(name)
  return setmetatable(self, Dog)
end

function Dog:sound()
  return "woof"
end

-- metamethods written as table fields, and a callable class
local Counter = setmetatable({}, {
  __call = function(cls, start)
    return cls.new(start)
  end,
  __tostring = function(self)
    return "Counter"
  end,
})

function Counter.new(start)
  return setmetatable({ n = start or 0 }, { __index = Counter })
end

function Counter:bump(by)
  self.n = self.n + (by or 1)
  return self
end

return { Animal = Animal, Dog = Dog, Counter = Counter }
