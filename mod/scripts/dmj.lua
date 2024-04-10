local Settings = {}
local State = "Stoped"
TheSim:GetPersistentString("dmj_bilibili", function(load_success, str)
    if load_success then
        local success, saved_settings = RunInSandbox(str)
        if success and saved_settings then
            Settings = saved_settings
        end
    end
end)

function DMJ_Save()
    local str = DataDumper(Settings, nil, true)
    TheSim:SetPersistentString("dmj_bilibili", str, false)
end

function DMJ_SetBaseURL(c)
    Settings.baseurl = c
    DMJ_Save()
    DMJ_Start()
end

function DMJ_SetRoomCode(c)
    Settings.roomcode = c
    DMJ_Save()
    DMJ_Start()
end

function DMJ_State()
    ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", "状态：" .. State, WHITE)
end

-- function DMJ_DisplaySettingPage()
--     local BilibiliSettingScreen = require "widgets/redux/bilibili"
--     local screen = BilibiliSettingScreen(Settings)
--     TheFrontEnd:PushScreen(screen)
-- end

function DMJ_Start()
    if not Settings.roomcode then
        ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", "未设置身份码", RED)
        return
    end
    ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", "已启动", WHITE)
    TheWorld:DoTaskInTime(0.1, DMJ_Fetch)
end

local nameColour = {}
nameColour[0] = UICOLOURS.WHITE
nameColour[1] = UICOLOURS.BLUE
nameColour[2] = UICOLOURS.GOLD
nameColour[3] = UICOLOURS.RED

function DMJ_Fetch()
    local baseurl = Settings.baseurl or "http://127.0.0.1:9876"
    TheSim:QueryServer(baseurl .. "/room/" .. Settings.roomcode .. "/msgs",
            function(result, isSuccessful, code)
                if isSuccessful and string.len(result) > 1 and code == 200 then
                    State = "Running"
                    local status, data = pcall(function() return json.decode(result) end)
                    if not status or not data then
                        return
                    end
                    for k, c in ipairs(data) do
                        if c.cmd == "LIVE_OPEN_PLATFORM_DM" then
                            local v = c.data
                            if v.uname and v.msg then
                                -- 是否增加几个mod选项，供选择头像等级组合？
                                if v.fans_medal_level and v.fans_medal_level ~= 0 and v.fans_medal_wearing_status then
                                    if v.fans_medal_level < 10 then
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg, nameColour[v.guard_level or 0],
                                                "profileflair_egg", nil, true) -- 鸟蛋
                                    elseif v.fans_medal_level < 20 then
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg, nameColour[v.guard_level or 0],
                                                "profileflair_crowkid", nil, true) -- 鸦年华小乌鸦
                                    else
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg, nameColour[v.guard_level or 0],
                                                "profileflair_corvus", nil, true) -- 鸦年华良羽鸦
                                    end
                                else
                                    ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg, nameColour[v.guard_level or 0],
                                            "profileflair_skincollector", nil, true)
                                end
                            end
                        elseif c.cmd == "LIVE_OPEN_PLATFORM_SEND_GIFT" then
                            -- 预留给
                        end
                    end
                    TheWorld:DoTaskInTime(0.1, DMJ_Fetch)
                else
                    State = "Error"
                end
            end)
end
