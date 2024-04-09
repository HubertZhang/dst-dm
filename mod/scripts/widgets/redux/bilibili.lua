local TEMPLATES = require "widgets/redux/templates"
local Widget = require "widgets/widget"
local Screen = require "widgets/screen"
local Text = require "widgets/text"
local PopupDialogScreen = require "screens/redux/popupdialog"

local BilibiliSettingScreen = Class(
    Screen,
    function(self, settings)
        Screen._ctor(self, "BilibiliSettingScreen")

        local data = {
            roomcode = settings.roomcode,
        }
        -- 背景
        self.black = self:AddChild(TEMPLATES.BackgroundTint())

        -- 屏幕根基
        self.root = self:AddChild(TEMPLATES.ScreenRoot())
        self.root:SetHAnchor(0)
        self.root:SetVAnchor(0)
        self.root:SetPosition(0, 0, 0)

        -- 位置参数
        local label_width = 300
        local spinner_width = 225
        local item_width, item_height = label_width + spinner_width + 30, 40

        -- 按键设置
        local buttons = {
            {
                text = "取消",
                cb = function()
                    TheFrontEnd:PopScreen(self)
                end
            },
            {
                text = "保存",
                cb = function()
                    settings.roomcode = data.roomcode
                    TheFrontEnd:PopScreen(self)
                end
            }
        }
        self.dialog = self.root:AddChild(TEMPLATES.RectangleWindow(item_width, 380, "Title?", buttons))
        -- 直播码填写框
        self.roomCode = self.root:AddChild(TEMPLATES.LabelTextbox("直播间用户码", data.roomcode or "", 120, 315, 40, 5, NEWFONT,
            25, -50))
        self.roomCode.textbox:SetTextLengthLimit(80)
        self.roomCode:SetPosition(0, 10)
        self.roomCode.textbox.OnTextInputted = function()
            data.roomcode = self.roomCode.textbox:GetString()
        end
    end)

    return BilibiliSettingScreen
