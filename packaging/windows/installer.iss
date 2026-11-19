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
CloseApplications=no
RestartApplications=no
SetupLogging=yes
ChangesAssociations=yes
UninstallDisplayName=MyGittu

[Languages]
Name: "zh_CN"; MessagesFile: "{#RepoRoot}\packaging\windows\languages\ChineseSimplified.isl"

[Tasks]
Name: desktopicon; Description: "创建桌面快捷方式"; Flags: unchecked

[Files]
Source: "dist\windows\x64\*"; DestDir: "{app}"; Excludes: "MyGittuShell.dll"; Flags: ignoreversion; Check: UseX64
Source: "dist\windows\arm64\*"; DestDir: "{app}"; Excludes: "MyGittuShell.dll"; Flags: ignoreversion; Check: UseArm64
Source: "dist\windows\x64\MyGittuShell.dll"; DestDir: "{app}"; DestName: "{#ShellFilename}"; Flags: ignoreversion restartreplace; Check: UseX64
Source: "dist\windows\arm64\MyGittuShell.dll"; DestDir: "{app}"; DestName: "{#ShellFilename}"; Flags: ignoreversion restartreplace; Check: UseArm64
Source: "packaging\windows\prepare-upgrade.ps1"; Flags: dontcopy
Source: "packaging\windows\restart-explorer.ps1"; Flags: ignoreversion; DestDir: "{app}"
Source: "packaging\windows\integration.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
#ifdef ModernShellPackage
Source: "{#ModernShellPackage}"; DestDir: "{app}"; DestName: "MyGittuShell.msix"
Source: "packaging\windows\register-modern.ps1"; DestDir: "{app}"
#endif
#ifdef ModernShellCertificate
Source: "{#ModernShellCertificate}"; DestDir: "{app}"; DestName: "MyGittu-Windows-Publisher.cer"; Flags: ignoreversion
#endif

[Registry]
#ifdef ModernShellThumbprint
Root: HKLM; Subkey: "Software\MyGittu"; ValueType: string; ValueName: "ShellPublisherThumbprint"; ValueData: "{#ModernShellThumbprint}"; Flags: uninsdeletevalue uninsdeletekeyifempty
#else
Root: HKLM; Subkey: "Software\MyGittu"; ValueType: none; ValueName: "ShellPublisherThumbprint"; Flags: deletevalue
#endif

[Icons]
Name: "{group}\MyGittu"; Filename: "{app}\desktop.exe"
Name: "{autodesktop}\MyGittu"; Filename: "{app}\desktop.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\desktop.exe"; Description: "启动 MyGittu"; Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File ""{app}\integration.ps1"" -Action Remove"; Flags: runhidden skipifdoesntexist; RunOnceId: MyGittuIntegration

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

function PrepareToInstall(var NeedsRestart: Boolean): String;
var ExitCode: Integer;
begin
  Result := '';
  if not (FileExists(ExpandConstant('{app}\agent.exe')) or FileExists(ExpandConstant('{app}\desktop.exe')) or
    FileExists(ExpandConstant('{app}\mygittu.exe'))) then
    Exit;
  ExtractTemporaryFile('prepare-upgrade.ps1');
  if not Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' +
    ExpandConstant('{tmp}\prepare-upgrade.ps1') + '" -InstallPath "' +
    ExpandConstant('{app}') + '\."', '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
    Result := '无法启动 MyGittu 进程关闭检查。'
  else if ExitCode <> 0 then
    Result := '无法关闭旧版 MyGittu 进程。请退出 MyGittu 窗口后重试。';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var ExitCode: Integer;
begin
  if (CurStep <> ssPostInstall) or not ExistingIntegration then
    Exit;
  if Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' +
    ExpandConstant('{app}\integration.ps1') + '" -Action Maintain',
    '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
    Log(Format('MyGittu existing integration maintenance exit: %d', [ExitCode]));
#ifdef ModernShellPackage
  if ExecAsOriginalUser(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "' +
    ExpandConstant('{app}\register-modern.ps1') + '" -IfRegistered -PackagePath "' +
    ExpandConstant('{app}\MyGittuShell.msix') + '" -InstallPath "' +
    ExpandConstant('{app}') + '\."', '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
    Log(Format('MyGittu existing modern menu maintenance exit: %d', [ExitCode]));
#endif
  if ExecAsOriginalUser(ExpandConstant('{app}\agent.exe'), '', '', SW_HIDE, ewNoWait, ExitCode) then
    Log('MyGittu Agent launch requested after upgrade');
end;