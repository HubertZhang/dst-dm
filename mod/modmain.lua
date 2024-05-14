GLOBAL.setmetatable(env, {
    __index = function(t, k) return GLOBAL.rawget(GLOBAL, k) end
})

require("dmj")

AddSimPostInit(function()
    DMJ_Start()
end)

