GLOBAL.setmetatable(env, {
    __index = function(t, k) return GLOBAL.rawget(GLOBAL, k) end
})

require("dmj")

AddSimPostInit(function()
    TheWorld:DoTaskInTime(0.1, DMJ_Start)
end)

