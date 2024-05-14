require "emoji_items"

local Settings = {}
local State = "Stoped"
local emojiDictionary = {}

local function LoadEmoji()
    local emoji_translator = {}
    for item_type, emoji in pairs(EMOJI_ITEMS) do
        if TheInventory:CheckOwnership(item_type) then
            emoji_translator[emoji.input_name] = emoji.data.utf8_str
        end
    end
    return emoji_translator
end



TheSim:GetPersistentString("dmj_bilibili", function(load_success, str)
    if load_success then
        local success, saved_settings = RunInSandbox(str)
        if success and saved_settings then
            Settings = saved_settings
        end
    end
end)

---@param text string
---@return string
local function replaceEmoji(text)
    local newText = ""
    local temp = nil
    for i = 1, text:len() do
        local c = text:sub(i, i)
        if temp == nil then
            if c == ":" then
                temp = ""
            else
                newText = newText .. c
            end
        else
            if c == ":" then
                -- print("find possible: " ..  temp)
                if emojiDictionary[temp] then
                    newText = newText .. emojiDictionary[temp]
                    temp = nil
                else
                    newText = newText .. ":" .. temp
                    temp = ""
                end
                
            else
                temp = temp .. c
            end
        end
    end
    if temp ~= nil then
        newText = newText .. ":" .. temp
    end
    return newText
end

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
    emojiDictionary = LoadEmoji()
    if not Settings.roomcode then
        Settings.roomcode = "-"
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
                                v.msg = replaceEmoji(v.msg)
                                -- 是否增加几个mod选项，供选择头像等级组合？
                                if v.fans_medal_level and v.fans_medal_level ~= 0 and v.fans_medal_wearing_status then
                                    if v.fans_medal_level < 10 then
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg,
                                            nameColour[v.guard_level or 0],
                                            "profileflair_egg", nil, true) -- 鸟蛋
                                    elseif v.fans_medal_level < 20 then
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg,
                                            nameColour[v.guard_level or 0],
                                            "profileflair_crowkid", nil, true) -- 鸦年华小乌鸦
                                    else
                                        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg,
                                            nameColour[v.guard_level or 0],
                                            "profileflair_corvus", nil, true) -- 鸦年华良羽鸦
                                    end
                                else
                                    ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, v.uname, v.msg,
                                        nameColour[v.guard_level or 0],
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
                    local error_msg = "【错误】请求结果："
                    if not isSuccessful then
                        error_msg = error_msg .. "请求失败"
                    else
                        error_msg = error_msg .. string.format("错误代码：%d， %s", code, result)
                    end
                    ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", error_msg, RED)
                end
            end)
end
