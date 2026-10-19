-- Calls: plain, method, qualified, Roblox services and instance-path requires.
local Players = game:GetService("Players")
local util = require(script.Parent.util)

local function helper() end

local function plain()
  helper()
  helper()
end

local function qualified()
  util.trim(" x ")
  string.format("%d", 1)
  workspace.Folder.Part:Destroy()
end

local function methods(obj: any)
  obj:run()
  obj.child:stop()
  Players.PlayerAdded:Connect(function(player: Player)
    print(player.Name)
  end)
end

local function sugar()
  log "message"
  configure { verbose = true }
end
