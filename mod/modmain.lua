GLOBAL.setmetatable(env, {
    __index = function(t, k) return GLOBAL.rawget(GLOBAL, k) end
})

GLOBAL.DMJ = {}
GLOBAL.DMJ.Configurations = {}

local options = {
    "show_enter"
}

for index, value in ipairs(options) do
    local m = GetModConfigData(value)
    GLOBAL.DMJ.Configurations[value] = m
end

require("dmj")

-- To use AddClassPostConstruct, you need to modimport it
modimport("scripts/gift_chatline.lua")

AddSimPostInit(function()
    DMJ_Start()
end)

