envFilePath := ".env"
spinnerHotkey := ""
apiSecret := ""
routerUrl := "http://100.112.36.101:8082" 

IfNotExist, %envFilePath%
{
    MsgBox, .env file not found at %envFilePath%
    ExitApp
}

Loop, Read, %envFilePath%
{
    if (A_LoopReadLine == "" || SubStr(A_LoopReadLine, 1, 1) == "#")
        continue
        
    if (InStr(A_LoopReadLine, "SPINNER_HOTKEY=") == 1) 
    {
        parts := StrSplit(A_LoopReadLine, "=")
        spinnerHotkey := parts[2]
    }
    
    ; Grab the secret from the .env file too!
    if (InStr(A_LoopReadLine, "HOTKEY_API_SECRET=") == 1) 
    {
        parts := StrSplit(A_LoopReadLine, "=")
        apiSecret := parts[2]
    }
}

if (spinnerHotkey != "") 
{
    Hotkey, %spinnerHotkey%, TriggerSpin
} 
else 
{
    MsgBox, SPINNER_HOTKEY not found in .env file.
    ExitApp
}

return

TriggerSpin:    
    ; Native AHK HTTP Request instead of spawning cmd.exe/curl
    try 
    {
        http := ComObjCreate("WinHttp.WinHttpRequest.5.1")
        http.Open("POST", routerUrl . "/api/spin", false)
        
        ; Inject the authorization header using the secret from the .env
        http.SetRequestHeader("Authorization", "Bearer " . apiSecret)
        
        http.Send()
    }
    catch e 
    {
        ; Optional: If you want to know if the desktop is offline/unreachable
        MsgBox, 16, Error, Failed to reach hotkey router. Is it running?
    }
return