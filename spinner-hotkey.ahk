envFilePath := ".env"
spinnerHotkey := ""

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
        break
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
    Run, cmd /k curl -v -X POST http://localhost:8082/api/spin,, Hide
return