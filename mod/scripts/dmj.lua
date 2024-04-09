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
    c_announce(State)
end

-- function DMJ_DisplaySettingPage()
--     local BilibiliSettingScreen = require "widgets/redux/bilibili"
--     local screen = BilibiliSettingScreen(Settings)
--     TheFrontEnd:PushScreen(screen)
-- end

function DMJ_Start()
    if not Settings.roomcode then
        return
    end
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
                        v = c.data
                        if v.uname and v.msg then
                            ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg, WHITE,
                                "profileflair_skincollector", nil, true)
                        end
                    end
                end
                TheWorld:DoTaskInTime(0.1, DMJ_Start)
            else
                State = "Error"
            end
        end)
end
