Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma
Name "MyGittu"
OutFile "dist\release\MyGittu-${AppVersion}-Windows-Setup.exe"
InstallDir "$PROGRAMFILES64\MyGittu"
RequestExecutionLevel admin
BrandingText " "
VIProductVersion "${AppVersion}.0"
VIAddVersionKey /LANG=2052 "ProductName" "MyGittu"
VIAddVersionKey /LANG=2052 "ProductVersion" "${AppVersion}"
VIAddVersionKey /LANG=2052 "FileDescription" "MyGittu 安装程序"
VIAddVersionKey /LANG=2052 "FileVersion" "${AppVersion}"
VIAddVersionKey /LANG=2052 "LegalCopyright" "Copyright 2026 MyGittu contributors"

!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!define UninstallKey "Software\Microsoft\Windows\CurrentVersion\Uninstall\{8E775DC6-289F-4CA1-9058-985C56602F39}_is1"
!define ShellDll "MyGittuShell.dll"
!define MUI_ICON "cmd\desktop\app.ico"
!define MUI_UNICON "cmd\desktop\app.ico"
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "启动 MyGittu"
!define MUI_FINISHPAGE_RUN_FUNCTION LaunchDesktop
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "创建桌面快捷方式"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Var PowerShell
Var PreviousDir
Var PreviousUninstaller
Var ExistingIntegration

!macro Init PREFIX
Function ${PREFIX}Init
  ${IfNot} ${RunningX64}
  ${OrIfNot} ${AtLeastWin10}
    MessageBox MB_ICONSTOP "MyGittu 需要 64 位 Windows 10 或更高版本。" /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  SetRegView 64
  SetShellVarContext all
  StrCpy $PowerShell "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists "$PowerShell" +2
  StrCpy $PowerShell "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
FunctionEnd

Function ${PREFIX}CloseRunning
  InitPluginsDir
  File /oname=$PLUGINSDIR\prepare-upgrade.ps1 "packaging\windows\prepare-upgrade.ps1"
  nsExec::ExecToLog /TIMEOUT=45000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$PLUGINSDIR\prepare-upgrade.ps1" -InstallPath "$INSTDIR\."'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONSTOP "无法关闭安装目录中的 MyGittu 进程。请退出 MyGittu 后重试。" /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
FunctionEnd
!macroend

!insertmacro Init ""
!insertmacro Init "un."

Function .onInit
  Call Init
  ReadRegStr $PreviousDir HKLM "${UninstallKey}" "InstallLocation"
  ReadRegStr $PreviousUninstaller HKLM "${UninstallKey}" "UninstallString"
  ${If} $PreviousDir != ""
    ; Inno records a trailing separator in InstallLocation.
    StrLen $0 $PreviousDir
    StrCpy $1 $PreviousDir 1 -1
    ${If} $0 > 3
    ${AndIf} $1 == "\"
      StrCpy $PreviousDir $PreviousDir -1
    ${EndIf}
    ; NSIS has already applied /D if the caller supplied it.
    ${If} $INSTDIR == "$PROGRAMFILES64\MyGittu"
      StrCpy $INSTDIR $PreviousDir
    ${EndIf}
  ${EndIf}
FunctionEnd

Function un.onInit
  Call un.Init
FunctionEnd

!macro InstallPayload ARCH
  File "dist\windows\${ARCH}\agent.exe"
  File "dist\windows\${ARCH}\desktop.exe"
  File "dist\windows\${ARCH}\mygittu.exe"
  File /oname=${ShellDll}.new "dist\windows\${ARCH}\MyGittuShell.dll"
!macroend

Section
  ${If} $PreviousDir != ""
  ${AndIf} $PreviousDir != $INSTDIR
    MessageBox MB_ICONSTOP "请使用现有安装目录 $PreviousDir，或先卸载旧版 MyGittu。" /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  ${If} ${FileExists} "$INSTDIR\${ShellDll}.new"
    MessageBox MB_ICONSTOP "MyGittu 外壳组件等待替换。请重启 Windows 后再运行安装程序。" /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
  Call CloseRunning
  StrCpy $ExistingIntegration 0
  StrCpy $1 1
  ${Do}
    ReadRegStr $0 HKLM "Software\Classes\CLSID\{29810E41-632A-4A9B-A190-4D659366810$1}\InprocServer32" ""
    StrCpy $2 "$INSTDIR\"
    StrLen $3 $2
    StrCpy $0 $0 $3
    ${If} $0 == $2
      StrCpy $ExistingIntegration 1
      ${ExitDo}
    ${EndIf}
    IntOp $1 $1 + 1
  ${LoopWhile} $1 <= 4

  ClearErrors
  SetOutPath "$INSTDIR"
  SetOverwrite on
  ${If} ${IsNativeARM64}
    !insertmacro InstallPayload arm64
  ${Else}
    !insertmacro InstallPayload x64
  ${EndIf}
  File "packaging\windows\restart-explorer.ps1"
  File "packaging\windows\integration.ps1"
  File "LICENSE"
  File "THIRD_PARTY_NOTICES.md"
  !ifdef ModernShellPackage
    File /oname=MyGittuShell.msix "${ModernShellPackage}"
    File "packaging\windows\register-modern.ps1"
  !endif
  !ifdef ModernShellCertificate
    File /oname=MyGittu-Windows-Publisher.cer "${ModernShellCertificate}"
  !endif
  IfErrors extractionFailed

  InitPluginsDir
  File /oname=$PLUGINSDIR\replace-shell.ps1 "packaging\windows\replace-shell.ps1"
  nsExec::ExecToLog /TIMEOUT=45000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$PLUGINSDIR\replace-shell.ps1" -InstallPath "$INSTDIR\."'
  Pop $0
  ${If} $0 != 0
    Goto extractionFailed
  ${EndIf}

  WriteUninstaller "$INSTDIR\Uninstall.exe"
  IfErrors installFailed
  !ifdef ModernShellThumbprint
    WriteRegStr HKLM "Software\MyGittu" "ShellPublisherThumbprint" "${ModernShellThumbprint}"
  !else
    DeleteRegValue HKLM "Software\MyGittu" "ShellPublisherThumbprint"
  !endif
  ClearErrors
  WriteRegStr HKLM "${UninstallKey}" "DisplayName" "MyGittu"
  WriteRegStr HKLM "${UninstallKey}" "DisplayVersion" "${AppVersion}"
  WriteRegStr HKLM "${UninstallKey}" "DisplayIcon" "$INSTDIR\desktop.exe"
  WriteRegStr HKLM "${UninstallKey}" "Publisher" "Oranjekop"
  WriteRegStr HKLM "${UninstallKey}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UninstallKey}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKLM "${UninstallKey}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKLM "${UninstallKey}" "NoModify" 1
  WriteRegDWORD HKLM "${UninstallKey}" "NoRepair" 1
  IfErrors installFailed
  CreateDirectory "$SMPROGRAMS\MyGittu"
  CreateShortCut "$SMPROGRAMS\MyGittu\MyGittu.lnk" "$INSTDIR\desktop.exe"
  ${If} $PreviousUninstaller == '"$INSTDIR\unins000.exe"'
    Delete "$INSTDIR\unins000.exe"
    Delete "$INSTDIR\unins000.dat"
  ${EndIf}

  ${If} $ExistingIntegration == 1
    nsExec::ExecToLog /TIMEOUT=45000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$INSTDIR\integration.ps1" -Action Maintain'
    Pop $0
    DetailPrint "MyGittu Explorer maintenance: $0"
    Push "Maintain"
    Call RunAsDesktopUser
  ${EndIf}
  Delete "$INSTDIR\launch-installed.ps1"
  nsExec::ExecToLog /TIMEOUT=120000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$INSTDIR\integration.ps1" -Action CleanupLegacy'
  Pop $0
  DetailPrint "MyGittu legacy shell cleanup: $0"
  Goto done

  extractionFailed:
    Delete "$INSTDIR\${ShellDll}.new"
  installFailed:
    MessageBox MB_ICONSTOP "无法写入 MyGittu 安装文件。请关闭正在使用这些文件的程序后重试。" /SD IDOK
    SetErrorLevel 2
    Abort
  done:
SectionEnd

Function RunAsDesktopUser
  Pop $0
  InitPluginsDir
  File /oname=$PLUGINSDIR\launch-installed.ps1 "packaging\windows\launch-installed.ps1"
  nsExec::ExecToLog /TIMEOUT=15000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$PLUGINSDIR\launch-installed.ps1" -InstallPath "$INSTDIR\." -Action $0'
  Pop $0
  DetailPrint "MyGittu desktop shell launch: $0"
FunctionEnd


Function LaunchDesktop
  Push "Launch"
  Call RunAsDesktopUser
FunctionEnd

Function CreateDesktopShortcut
  CreateShortCut "$DESKTOP\MyGittu.lnk" "$INSTDIR\desktop.exe"
FunctionEnd

Section "Uninstall"
  Call un.CloseRunning
  ${If} ${FileExists} "$INSTDIR\integration.ps1"
    nsExec::ExecToLog /TIMEOUT=45000 '"$PowerShell" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$INSTDIR\integration.ps1" -Action Remove'
    Pop $0
    ${If} $0 != 0
      MessageBox MB_ICONSTOP "无法清理 MyGittu 的资源管理器集成。请重试卸载。" /SD IDOK
      SetErrorLevel 2
      Abort
    ${EndIf}
  ${EndIf}
  Delete "$SMPROGRAMS\MyGittu\MyGittu.lnk"
  RMDir "$SMPROGRAMS\MyGittu"
  Delete "$DESKTOP\MyGittu.lnk"
  DeleteRegKey HKLM "${UninstallKey}"
  DeleteRegValue HKLM "Software\MyGittu" "ShellPublisherThumbprint"
  DeleteRegKey /ifempty HKLM "Software\MyGittu"
  Delete /REBOOTOK "$INSTDIR\MyGittuShell-*.dll"
  Delete /REBOOTOK "$INSTDIR\${ShellDll}"
  Delete /REBOOTOK "$INSTDIR\${ShellDll}.retired-*"
  Delete /REBOOTOK "$INSTDIR\${ShellDll}.new"
  Delete "$INSTDIR\desktop.exe"
  Delete "$INSTDIR\agent.exe"
  Delete "$INSTDIR\mygittu.exe"
  Delete "$INSTDIR\integration.ps1"
  Delete "$INSTDIR\launch-installed.ps1"
  Delete "$INSTDIR\restart-explorer.ps1"
  Delete "$INSTDIR\register-modern.ps1"
  Delete "$INSTDIR\MyGittuShell.msix"
  Delete "$INSTDIR\MyGittu-Windows-Publisher.cer"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\THIRD_PARTY_NOTICES.md"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir /REBOOTOK "$INSTDIR"
SectionEnd
