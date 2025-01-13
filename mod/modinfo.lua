name = "弹幕机"
priority = 1
description = ""
author = "Hubert Zhang"
version = "0.6"
forumthread = ""
-- icon_atlas = "modicon.xml"
-- icon = "modicon.tex"
dst_compatible = true
client_only_mod = true
all_clients_require_mod = false
api_version = 10

configuration_options = {
    {
        name = "show_enter",
        label = "进入直播间提示",
        hover = "是否显示用户进入直播间提示",
        options = {
            { description = "显示", data = "true" },
            { description = "隐藏", data = "false" }
        },
        default = "true",
    }
}
