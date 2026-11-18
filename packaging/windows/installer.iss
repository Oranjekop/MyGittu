#ifndef AppVersion
  #define AppVersion "0.0.2"
#endif
#define RepoRoot AddBackslash(SourcePath) + "..\.."

#define ShellFilename "MyGittuShell-" + AppVersion + ".dll"

[Setup]
AppId={{8E775DC6-289F-4CA1-9058-985C56602F39}
AppName=MyGittu
AppVersion={#AppVersion}
AppPublisher=Oranjekop
AppPublisherURL=https://github.com/Oranjekop/MyGittu
AppSupportURL=https://github.com/Oranjekop/MyGittu/issues
SourceDir={#RepoRoot}
DefaultDirName={autopf}\MyGittu
DefaultGroupName=MyGittu
UninstallDisplayIcon={app}\desktop.exe
SetupIconFile=cmd\desktop\app.ico
OutputDir=dist\release
OutputBaseFilename=MyGittu-{#AppVersion}-Windows-Setup
ArchitecturesAllowed=x64compatible or arm64
ArchitecturesInstallIn64BitMode=x64compatible or arm64
MinVersion=10.0
PrivilegesRequired=admin
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ShowLanguageDialog=no
LanguageDetectionMethod=none
LicenseFile=LICENSE
CloseApplications=yes
CloseApplicationsFilter=desktop.exe,agent.exe,mygittu.exe
RestartApplications=no
SetupLogging=yes
ChangesAssociations=yes
UninstallDisplayName=MyGittu

[Languages]
Name: "zh_CN"; MessagesFile: "{#RepoRoot}\packaging\windows\languages\ChineseSimplified.isl"

[Tasks]
Name: desktopicon; Description: "创建桌面快捷方式"; Flags: unchecked
Name: shell; Description: "启用资源管理器右键菜单"; Flags: unchecked
Name: overlays; Description: "启用 Git 文件状态图标"; Flags: unchecked

[Files]
Source: "dist\windows\x64\*"; DestDir: "{app}"; Excludes: "MyGittuShell.dll"; Flags: ignoreversion; Check: UseX64
Source: "dist\windows\arm64\*"; DestDir: "{app}"; Excludes: "MyGittuShell.dll"; Flags: ignoreversion; Check: UseArm64
Source: "dist\windows\x64\MyGittuShell.dll"; DestDir: "{app}"; DestName: "{#ShellFilename}"; Flags: ignoreversion restartreplace; Check: UseX64
Source: "dist\windows\arm64\MyGittuShell.dll"; DestDir: "{app}"; DestName: "{#ShellFilename}"; Flags: ignoreversion restartreplace; Check: UseArm64
Source: "packaging\windows\prepare-upgrade.ps1"; Flags: dontcopy
Source: "packaging\windows\restart-explorer.ps1"; Flags: dontcopy
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
#ifdef ModernShellPackage
Source: "{#ModernShellPackage}"; DestDir: "{app}"; DestName: "MyGittuShell.msix"; Tasks: shell
Source: "packaging\windows\register-modern.ps1"; DestDir: "{app}"; Tasks: shell
#endif

[Icons]
Name: "{group}\MyGittu"; Filename: "{app}\desktop.exe"
Name: "{autodesktop}\MyGittu"; Filename: "{app}\desktop.exe"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "MyGittuAgent"; ValueData: """{app}\agent.exe"""; Flags: uninsdeletevalue; Check: RestartExplorer
Root: HKLM; Subkey: "Software\Classes\CLSID\{{29810E41-632A-4A9B-A190-4D6593668101}\InprocServer32"; ValueType: string; ValueData: "{app}\{#ShellFilename}"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\CLSID\{{29810E41-632A-4A9B-A190-4D6593668101}\InprocServer32"; ValueType: string; ValueName: ThreadingModel; ValueData: Apartment; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\*\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\Directory\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\Directory\Background\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell

[Run]
Filename: "{app}\desktop.exe"; Description: "启动 MyGittu"; Flags: nowait postinstall skipifsilent runasoriginaluser

#ifdef ModernShellPackage
[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File ""{app}\register-modern.ps1"" -Remove"; Flags: runhidden skipifdoesntexist; RunOnceId: MyGittuModernMenu
#endif

[Code]
function UseArm64: Boolean;
begin Result := IsArm64; end;
function UseX64: Boolean;
begin Result := not IsArm64; end;

function ExistingIntegration: Boolean;
var I: Integer; Value, Prefix: String;
begin
  Result := False;
  Prefix := AddBackslash(ExpandConstant('{app}'));
  for I := 1 to 4 do
    if RegQueryStringValue(HKLM, 'Software\Classes\CLSID\{29810E41-632A-4A9B-A190-4D659366810' +
      IntToStr(I) + '}\InprocServer32', '', Value) and
      (CompareText(Copy(Value, 1, Length(Prefix)), Prefix) = 0) then
    begin
      Result := True;
      Exit;
    end;
end;

function RestartExplorer: Boolean;
begin
  Result := WizardIsTaskSelected('shell') or WizardIsTaskSelected('overlays') or ExistingIntegration;
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var ExitCode: Integer;
begin
  Result := '';
  if not FileExists(ExpandConstant('{app}\agent.exe')) then
    Exit;
  ExtractTemporaryFile('prepare-upgrade.ps1');
  if not Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' +
    ExpandConstant('{tmp}\prepare-upgrade.ps1') + '" -InstallPath "' +
    ExpandConstant('{app}') + '\."', '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
    Result := '无法停止旧版 MyGittu 后台服务。'
  else if ExitCode <> 0 then
    Result := '旧版 MyGittu 后台服务仍在运行。请关闭后重试。';
end;

procedure RegisterOverlays;
var I: Integer; Key, Id, LabelText: String;
begin
  if WizardIsTaskSelected('overlays') then
    for I := 2 to 4 do begin
      Id := '{29810E41-632A-4A9B-A190-4D659366810' + IntToStr(I) + '}';
      Key := 'Software\Classes\CLSID\' + Id + '\InprocServer32';
      if not RegWriteStringValue(HKLM, Key, '', ExpandConstant('{app}\{#ShellFilename}')) then
        Log('MyGittu could not register overlay DLL: ' + Id);
      if not RegWriteStringValue(HKLM, Key, 'ThreadingModel', 'Apartment') then
        Log('MyGittu could not register overlay threading model: ' + Id);
      case I of 2: LabelText := 'MyGittu Clean'; 3: LabelText := 'MyGittu Modified'; 4: LabelText := 'MyGittu Conflict'; end;
      if not RegWriteStringValue(HKLM, 'Software\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers\' + LabelText, '', Id) then
        Log('MyGittu could not register overlay identifier: ' + LabelText);
    end;
end;

function RunUserPowerShell(const Arguments, StepName: String): Boolean;
var ExitCode: Integer; Started: Boolean;
begin
  ExitCode := -1;
  Started := ExecAsOriginalUser(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File ' + Arguments,
    '', SW_HIDE, ewWaitUntilTerminated, ExitCode);
  Log(Format('MyGittu %s: started=%d, exit=%d', [StepName, Ord(Started), ExitCode]));
  Result := Started and (ExitCode = 0);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var ExitCode: Integer;
#ifdef ModernShellPackage
    Version: TWindowsVersion;
#endif
begin
  if CurStep <> ssPostInstall then
    Exit;
  RegisterOverlays;
  if not RestartExplorer then
    Exit;
  if not ExecAsOriginalUser(ExpandConstant('{app}\agent.exe'), '', '',
    SW_HIDE, ewNoWait, ExitCode) then
    Log(Format('MyGittu Agent launch failed: %d', [ExitCode]))
  else
    Log('MyGittu Agent launch requested');
#ifdef ModernShellPackage
  GetWindowsVersionEx(Version);
  if (Version.Build >= 22000) and WizardIsTaskSelected('shell') then
    if not RunUserPowerShell('"' + ExpandConstant('{app}\register-modern.ps1') +
      '" -PackagePath "' + ExpandConstant('{app}\MyGittuShell.msix') +
      '" -InstallPath "' + ExpandConstant('{app}') + '\."', 'modern menu registration') then
      if not WizardSilent then
        SuppressibleMsgBox('Windows 11 新版右键菜单注册失败。传统入口仍可从“显示更多选项”打开。',
          mbInformation, MB_OK, IDOK);
#endif
  ExtractTemporaryFile('restart-explorer.ps1');
  WizardForm.StatusLabel.Caption := '正在重启 Windows 资源管理器…';
  if not RunUserPowerShell('"' + ExpandConstant('{tmp}\restart-explorer.ps1') + '"', 'Explorer refresh') then
    if not WizardSilent then
      SuppressibleMsgBox('MyGittu 已安装。Windows 资源管理器未能自动重启，请注销并重新登录，以加载新版右键菜单和状态图标。',
        mbInformation, MB_OK, IDOK);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var I: Integer; Id, LabelText: String;
begin
  if CurUninstallStep = usUninstall then
    for I := 1 to 4 do begin
      Id := '{29810E41-632A-4A9B-A190-4D659366810' + IntToStr(I) + '}';
      RegDeleteKeyIncludingSubkeys(HKLM, 'Software\Classes\CLSID\' + Id);
      if I > 1 then begin
        case I of 2: LabelText := 'MyGittu Clean'; 3: LabelText := 'MyGittu Modified'; 4: LabelText := 'MyGittu Conflict'; end;
        RegDeleteKeyIncludingSubkeys(HKLM, 'Software\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers\' + LabelText);
      end;
    end;
end;
