-- PalHelperBot: команды из глобального чата сервера
-- "pal <имя>" — поиск пала, "кто онлайн" — список игроков
-- Запросы пишет в мост: C:/dev/pal-helper/bridge/in

local BRIDGE_IN = "C:/dev/pal-helper/bridge/in"

local seq = 0

local function writeQuery(q)
    seq = seq + 1
    local path = string.format("%s/q%d_%d.txt", BRIDGE_IN, os.time(), seq)
    local f = io.open(path, "w")
    if f then
        f:write(q)
        f:close()
    end
end

RegisterHook("/Script/Pal.PalGameStateInGame:BroadcastChatMessage", function(self, ChatMessage)
    local ok, err = pcall(function()
        local msg = ChatMessage:get()
        local sender = msg.Sender:ToString()
        local text = msg.Message:ToString()

        local low = string.lower(text)
        if string.sub(text, 1, 4) == "pal " then
            print("[PalHelperBot] запрос от " .. sender .. ": " .. text)
            writeQuery(string.sub(text, 5))
        elseif string.find(low, "кто онлайн", 1, true) then
            print("[PalHelperBot] запрос онлайна от " .. sender)
            writeQuery("кто онлайн")
        end
    end)
    if not ok then
        print("[PalHelperBot] ошибка: " .. tostring(err))
    end
end)
