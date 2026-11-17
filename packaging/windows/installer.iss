#ifndef AppVersion
  #define AppVersion "0.0.2"
#endif
#define RepoRoot AddBackslash(SourcePath) + "..\.."

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
RestartApplications=no
ChangesAssociations=yes
UninstallDisplayName=MyGittu

[Languages]
Name: "zh_CN"; MessagesFile: "{#RepoRoot}\packaging\windows\languages\ChineseSimplified.isl"

[Tasks]
Name: desktopicon; Description: "创建桌面快捷方式"; Flags: unchecked
Name: shell; Description: "启用资源管理器右键菜单"; Flags: unchecked
Name: overlays; Description: "启用 Git 文件状态图标（可能需要重新启动资源管理器）"; Flags: unchecked

[Files]
Source: "dist\windows\x64\*"; DestDir: "{app}"; Flags: ignoreversion; Check: UseX64
Source: "dist\windows\arm64\*"; DestDir: "{app}"; Flags: ignoreversion; Check: UseArm64
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\MyGittu"; Filename: "{app}\desktop.exe"
Name: "{autodesktop}\MyGittu"; Filename: "{app}\desktop.exe"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "Software\Classes\CLSID\{{29810E41-632A-4A9B-A190-4D6593668101}\InprocServer32"; ValueType: string; ValueData: "{app}\MyGittuShell.dll"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\CLSID\{{29810E41-632A-4A9B-A190-4D6593668101}\InprocServer32"; ValueType: string; ValueName: ThreadingModel; ValueData: Apartment; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\*\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\Directory\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell
Root: HKLM; Subkey: "Software\Classes\Directory\Background\shellex\ContextMenuHandlers\MyGittu"; ValueType: string; ValueData: "{{29810E41-632A-4A9B-A190-4D6593668101}"; Flags: uninsdeletekey; Tasks: shell

[Run]
Filename: "{app}\desktop.exe"; Description: "启动 MyGittu"; Flags: nowait postinstall skipifsilent runasoriginaluser

[Code]
function UseArm64: Boolean;
begin Result := IsArm64; end;
function UseX64: Boolean;
begin Result := not IsArm64; end;

procedure CurStepChanged(CurStep: TSetupStep);
var I: Integer; Key, Id, LabelText: String;
begin
  if (CurStep = ssPostInstall) and WizardIsTaskSelected('overlays') then
    for I := 2 to 4 do begin
      Id := '{29810E41-632A-4A9B-A190-4D659366810' + IntToStr(I) + '}';
      Key := 'Software\Classes\CLSID\' + Id + '\InprocServer32';
      RegWriteStringValue(HKLM, Key, '', ExpandConstant('{app}\MyGittuShell.dll'));
      RegWriteStringValue(HKLM, Key, 'ThreadingModel', 'Apartment');
      case I of 2: LabelText := 'MyGittu Clean'; 3: LabelText := 'MyGittu Modified'; 4: LabelText := 'MyGittu Conflict'; end;
      RegWriteStringValue(HKLM, 'Software\Microsoft\Windows\CurrentVersion\Explorer\ShellIconOverlayIdentifiers\' + LabelText, '', Id);
    end;
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
