#NoEnv
#SingleInstance Force
SendMode Input
SetWorkingDir %A_ScriptDir%

envPath := A_ScriptDir . "\.env"
configPath := A_ScriptDir . "\hotkeys.json"
apiSecret := ""
desktopIP := ""

if !FileExist(envPath)
{
    MsgBox, 16, Hotkey Listener, .env file not found at %envPath%.
    ExitApp
}

Loop, Read, %envPath%
{
    line := Trim(A_LoopReadLine)
    if (line = "" || SubStr(line, 1, 1) = "#")
        continue

    if (InStr(line, "HOTKEY_API_SECRET=") = 1)
        apiSecret := UnquoteEnvValue(Trim(SubStr(line, StrLen("HOTKEY_API_SECRET=") + 1)))
    else if (InStr(line, "DESKTOP_IP=") = 1)
        desktopIP := UnquoteEnvValue(Trim(SubStr(line, StrLen("DESKTOP_IP=") + 1)))
}

if (apiSecret = "")
{
    MsgBox, 16, Hotkey Listener, HOTKEY_API_SECRET not found or empty in .env.
    ExitApp
}

if (desktopIP = "")
{
    MsgBox, 16, Hotkey Listener, DESKTOP_IP not found or empty in .env.
    ExitApp
}

if !FileExist(configPath)
{
    MsgBox, 16, Hotkey Listener, Hotkey configuration not found at %configPath%.
    ExitApp
}

FileRead, hotkeysJson, %configPath%
if (ErrorLevel)
{
    MsgBox, 16, Hotkey Listener, Failed to read %configPath%.
    ExitApp
}

hotkeyEndpoints := {}
quote := Chr(34)
pairPattern := quote . "([^" . quote . "]*)" . quote . "\s*:\s*" . quote . "([^" . quote . "]*)" . quote
position := 1
hotkeyCount := 0

while (position := RegExMatch(hotkeysJson, pairPattern, match, position))
{
    hotkey := match1
    endpoint := match2
    if (hotkey = "" || endpoint = "")
    {
        MsgBox, 16, Hotkey Listener, Hotkey keys and endpoint values must not be empty.
        ExitApp
    }

    StringLower, hotkeyLookupKey, hotkey
    hotkeyEndpoints[hotkeyLookupKey] := endpoint
    Hotkey, %hotkey%, TriggerHotkey
    if (ErrorLevel)
    {
        MsgBox, 16, Hotkey Listener, Failed to register hotkey: %hotkey%.
        ExitApp
    }

    hotkeyCount++
    position += StrLen(match)
}

if (hotkeyCount = 0)
{
    MsgBox, 16, Hotkey Listener, No hotkey-to-endpoint pairs found in %configPath%.
    ExitApp
}

apiBase := "http://" . desktopIP . ":8083"
return

TriggerHotkey:
    StringLower, hotkeyLookupKey, A_ThisHotkey
    endpoint := hotkeyEndpoints[hotkeyLookupKey]
    SendHotkeyRequest(endpoint, apiBase, apiSecret)
return

SendHotkeyRequest(endpoint, apiBase, apiSecret)
{
    try
    {
        http := ComObjCreate("WinHttp.WinHttpRequest.5.1")
        http.Open("POST", apiBase . "/" . endpoint, false)
        http.SetRequestHeader("Authorization", "Bearer " . apiSecret)
        http.Send()

        if (http.Status < 200 || http.Status >= 300)
        {
            status := http.Status
            MsgBox, 16, Hotkey Listener, Request for %endpoint% failed with HTTP status %status%.
        }
    }
    catch e
    {
        MsgBox, 16, Hotkey Listener, Failed to send the request to %apiBase%.
    }
}

UnquoteEnvValue(value)
{
    first := SubStr(value, 1, 1)
    last := SubStr(value, 0)
    if ((first = Chr(34) && last = Chr(34)) || (first = Chr(39) && last = Chr(39)))
        return SubStr(value, 2, StrLen(value) - 2)
    return value
}
