local TEMPLATES = require "widgets/redux/templates"

-- 游戏内聊天消息
AddClassPostConstruct("widgets/redux/chatline", function(chatline)
    local oldSetChatData = chatline.SetChatData
    function chatline:SetChatData(mtype, alpha, message, m_colour, sender, s_colour, icondata, icondatabg)
        -- print("SetChatData", type, alpha, message, m_colour, sender, s_colour, icondata, icondatabg)
        if mtype == 9 then
            self.type = ChatTypes.SkinAnnouncement
            self.skin_data = nil

            if alpha > 0 then
                self.root:Show()
                self.message:Hide()
                self.user:Hide()

                self.skin_btn:Show()
                self.skin_txt:Show()

                self.skin_btn:SetText("收到来自 " .. sender .. " 的 ")
                self.skin_btn.text:UpdateAlpha(alpha)

                local r, g, b = unpack(UICOLOURS.GOLD_FOCUS)
                self.skin_txt:SetColour(r, g, b, alpha)
                self.skin_txt:SetString(message)

                self:UpdateSkinAnnouncementPosition()
                self.flair:Hide()
                self.announcement:Hide()
                self.systemmessage:Hide()
                self.chattermessage:Hide()

                self.announcement:SetAnnouncement("item_drop")
                self.announcement:SetAlpha(alpha)
            else
                self.root:Hide()
            end
        else
            return oldSetChatData(self, mtype, alpha, message, m_colour, sender, s_colour, icondata, icondatabg)
        end
    end
end)

-- 大厅聊天消息
AddClassPostConstruct("widgets/redux/lobbychatline", function(self)
    -- print("lobby", chatline)
    if self.type == 9 then
        self.icon = self.root:AddChild(TEMPLATES.AnnouncementBadge())
        self.icon:SetAnnouncement("item_drop")
        self.message:SetString("投喂 " .. self.message:GetString())
        self.inital_update = false
        self:UpdatePositions()
    end
end)
