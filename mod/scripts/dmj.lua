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
    coroutine.resume(coroutine.create(DMJ_Fetch))
end

-- 对应房间大航海 1总督 2提督 3舰长
-- TODO: 增加mod选项，供选择等级颜色
local guardLevelToColour = {
    [0] = UICOLOURS.WHITE,
    [1] = UICOLOURS.RED,
    [2] = UICOLOURS.GOLD,
    [3] = UICOLOURS.BLUE,
}

local function levelToIcon(level)
    -- TODO: 增加几个mod选项，供选择等级头像
    if level == 0 then
        return "profileflair_skincollector" -- 皮肤收集者
    elseif level < 10 then
        return "profileflair_egg"           -- 鸟蛋
    elseif level < 20 then
        return "profileflair_crowkid"       -- 小乌鸦
    else
        return "profileflair_corvus"        -- 良羽鸦
    end
end

local function handleDM(data)
    if data.uname and data.msg then
        data.msg = replaceEmoji(data.msg)
        local level = 0
        if data.fans_medal_level and data.fans_medal_level ~= 0 and data.fans_medal_wearing_status then
            level = data.fans_medal_level
        end
        local icon = levelToIcon(level)
        ChatHistory:AddToHistory(ChatTypes.Message, nil, nil, data.uname, data.msg,
            guardLevelToColour[data.guard_level or 0],
            icon, nil, true)
    end
end

local seenUsers = {}

local function expireSeenUsers(t)
    -- t 的单位是秒
    for k, v in pairs(seenUsers) do
        if v < t - 600 then
            seenUsers[k] = nil
        end
    end
end

local function handleEnter(data)
    if DMJ.Configurations["show_enter"] ~= "true" then
        return
    end
    if not data then
        return
    end
    if data.openid and not data.open_id then
        data.open_id = data.openid
    end
    if not data.uname or not data.open_id or not data.timestamp then
        return
    end
    expireSeenUsers(data.timestamp)
    -- 这里设定重复进入的用户不会被提示
    if seenUsers[data.open_id] then
        print(string.format("用户 %s 已在直播间, 上次进入时间 %d, 本次进入时间 %d", data.uname, seenUsers[data.open_id], data.timestamp))
        seenUsers[data.open_id] = data.timestamp
        return
    end
    seenUsers[data.open_id] = data.timestamp
    ChatHistory:AddToHistory(ChatTypes.Announcement, nil, nil, nil, data.uname .. " 进入了直播间。", UICOLOURS.WHITE,
        "join_game", nil, true)
end


-- 字段名	类型	描述
-- room_id	int64	房间号
-- uid	int64	用户UID（已废弃，固定为0）
-- open_id	string	用户唯一标识
-- uname	string	送礼用户昵称
-- uface	string	送礼用户头像
-- gift_id	int64	道具id(盲盒:爆出道具id)
-- gift_name	string	道具名(盲盒:爆出道具名)
-- gift_num	int64	赠送道具数量
-- price	int64	礼物爆出单价，(1000 = 1元 = 10电池),盲盒:爆出道具的价值
-- r_price	int64	实际价值(1000 = 1元 = 10电池),盲盒:爆出道具的价值
-- paid	bool	是否是付费道具
-- fans_medal_level	int64	实际送礼人的勋章信息
-- fans_medal_name	string	粉丝勋章名
-- fans_medal_wearing_status	bool	该房间粉丝勋章佩戴情况
-- guard_level	int64	大航海等级
-- timestamp	int64	收礼时间秒级时间戳
-- anchor_info	anchor_info结构体	主播信息
-- msg_id	string	消息唯一id
-- gift_icon	string	道具icon
-- combo_gift	bool	是否是combo道具
-- combo_info	combo_info结构体	连击信息

-- anchor_info结构体	类型	描述
-- uid	int64	收礼主播uid
-- open_id	string	收礼主播唯一标识(2024-03-11后上线)
-- uname	string	收礼主播昵称
-- uface	string	收礼主播头像
-- combo_info	类型	描述
-- combo_base_num	int64	每次连击赠送的道具数量
-- combo_count	int64	连击次数
-- combo_id	string	连击id
-- combo_timeout	int64	连击有效期秒
local function handleGift(data)
    if data.uname then
        local msg = data.gift_num .. " 个 " .. data.gift_name
        ChatHistory:AddToHistory(9, nil, nil, data.uname, msg,
            UICOLOURS.GOLD_FOCUS,
            "item_drop", nil, true)
    end
end

function DMJ_Fetch()
    local baseurl = Settings.baseurl or "http://127.0.0.1:9876"
    TheSim:QueryServer(baseurl .. "/room/" .. Settings.roomcode .. "/msgs",
        function(result, isSuccessful, code)
            if isSuccessful and string.len(result) > 1 and code == 200 then
                State = "Running"
                local status, data = pcall(function() return json.decode(result) end)
                if not status or not data then
                    State = "Error"
                    local error_msg = "【错误】请求解析失败："
                    error_msg = error_msg .. result
                    ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", error_msg, UICOLOURS.RED)
                    return
                end
                for k, c in ipairs(data) do
                    -- print(c.cmd, c.data)
                    if c.cmd == "LIVE_OPEN_PLATFORM_DM" then
                        handleDM(c.data)
                    elseif c.cmd == "LIVE_OPEN_PLATFORM_SEND_GIFT" then
                        handleGift(c.data)
                    elseif c.cmd == "LIVE_OPEN_PLATFORM_LIVE_ROOM_ENTER" then
                        handleEnter(c.data)
                    end
                end
                coroutine.resume(coroutine.create(DMJ_Fetch))
            else
                State = "Error"
                local error_msg = "【错误】请求结果："
                if not isSuccessful then
                    error_msg = error_msg .. "请求失败"
                else
                    error_msg = error_msg .. string.format("错误代码：%d， %s", code, result)
                end
                ChatHistory:AddToHistory(ChatTypes.SystemMessage, nil, nil, "弹幕机", error_msg, UICOLOURS.RED)
            end
        end)
end

local testData = {
    {
        gift_id = 31164,
        gift_name = "粉丝团灯牌",
        gift_num = 1,
        price = 100,
        paid = true,
        gift_icon = "https://s1.hdslb.com/bfs/live/e051dfd4557678f8edcac4993ed00a0935cbd9cc.png",
        r_price = 100,
    },
    {
        gift_id = 33988,
        gift_name = "人气票",
        gift_num = 1,
        price = 100,
        paid = true,
        gift_icon = "https://s1.hdslb.com/bfs/live/7164c955ec0ed7537491d189b821cc68f1bea20d.png",
        r_price = 100,
    },
    {
        gift_id = 31212,
        gift_name = "打call",
        gift_num = 1,
        price = 500,
        paid = true,
        gift_icon = "https://s1.hdslb.com/bfs/live/461be640f60788c1d159ec8d6c5d5cf1ef3d1830.png",
        r_price = 500,
    }
}

function TestGift(id)
    local data = {
        gift_id = 31164,
        gift_name = "粉丝团灯牌",
        gift_num = 1,
        price = 100,
        paid = true,
        gift_icon = "https://s1.hdslb.com/bfs/live/e051dfd4557678f8edcac4993ed00a0935cbd9cc.png",
        r_price = 100,
    }
    if id then
        data = testData[id % 3 + 1]
    end
    handleGift({
        room_id = 1, -- 直播间(演播厅模式则为演播厅直播间,非演播厅模式则为收礼直播间)
        uid = 0, -- 用户UID(已废弃，固定为0)
        open_id = "39b8fedb-60a5-4e29-ac75-b16955f7e632", -- 用户唯一标识
        uname = "用户昵称", -- 送礼用户昵称
        uface = "", -- 送礼用户头像
        fans_medal_level = 15, -- 实际收礼人的勋章信息
        fans_medal_name = "粉丝勋章名", -- 粉丝勋章名
        fans_medal_wearing_status = true, -- 该房间粉丝勋章佩戴情况
        guard_level = 0, -- room_id对应的大航海等级
        timestamp = 1736229854, -- 收礼时间秒级时间戳
        msg_id = "da5fc51b-4c63-4e6d-8e34-53f9b0ef1005", -- 消息唯一id
        anchor_info = {
            uid = 0, -- 收礼主播UID(即将废弃)
            open_id = "39b8fedb-60a5-4e29-ac75-b16955f7e632", -- 主播唯一标识(2024-03-11后上线)
            uname = "", -- 收礼主播昵称
            uface = "http://i0.hdslb.com/bfs/face/4add3acfc930fcd07d06ea5e10a3a377314141c2.jpg" -- 收礼主播头像
        },
        combo_gift = true, -- 是否是combo道具
        combo_info = { -- ex：连击次数100，每个连击是批量送5个 既  5 * 100
            combo_base_num = 1, -- 每次连击赠送的道具数量
            combo_count = 1, -- 连击次数
            combo_id = "batch:gift:combo_id:693408872:535852715:33988:1736229854.1947", -- 连击id
            combo_timeout = 5, -- 连击有效期秒
        },
        gift_id = data.gift_id,
        gift_name = data.gift_name,
        gift_num = data.gift_num,
        price = data.price,
        paid = data.paid,
        gift_icon = data.gift_icon,
        r_price = data.r_price,
    })
end
