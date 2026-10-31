; Open Charts installer (Inno Setup 6). Build with installer\build.ps1.
; Per-user install: no administrator rights needed.

#define AppName "Open Charts"
#ifndef AppVersion
  #define AppVersion "0.8.0"
#endif
#ifndef AppLabel
  #define AppLabel "Open Beta"
#endif

[Setup]
AppId={{8F3C2A55-7E6B-4C0B-9B8E-0C6F5C2D1A10}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion} {#AppLabel}
VersionInfoVersion={#AppVersion}
AppPublisher=Rafael Coelho
DefaultDirName={localappdata}\Programs\OpenCharts
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=OpenCharts-Setup-{#AppVersion}-open-beta
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
LicenseFile=..\LICENSE
UninstallDisplayIcon={app}\opencharts.exe
CloseApplications=no

[Languages]
Name: "ptbr"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[CustomMessages]
ptbr.TaskEfb=Instalar o app no EFB do Microsoft Flight Simulator
en.TaskEfb=Install the app in the Microsoft Flight Simulator EFB
ptbr.TaskAutostart=Iniciar o Open Charts junto com o MSFS
en.TaskAutostart=Start Open Charts together with MSFS
ptbr.RunOpen=Abrir o Open Charts agora
en.RunOpen=Open Open Charts now
ptbr.Disclaimer=Apenas para simulação aérea. É vedado o uso em aviação real.
en.Disclaimer=For flight simulation only. Use in real-world aviation is prohibited.
ptbr.AskDeleteData=Apagar também a biblioteca de cartas e os grupos (%1)?%n%nEscolha "Não" para manter seus dados.
en.AskDeleteData=Also delete the chart library and groups (%1)?%n%nChoose "No" to keep your data.

[Tasks]
Name: "efb"; Description: "{cm:TaskEfb}"; GroupDescription: "MSFS"
Name: "autostart"; Description: "{cm:TaskAutostart}"; GroupDescription: "MSFS"
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Files]
Source: "..\build\opencharts.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\SimConnect.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\opencharts.exe"; Parameters: "-open"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\opencharts.exe"; Parameters: "-open"; Tasks: desktopicon

[Run]
Filename: "{app}\opencharts.exe"; Parameters: "setup install-efb"; Tasks: efb; Flags: runhidden waituntilterminated
Filename: "{app}\opencharts.exe"; Parameters: "setup autostart on"; Tasks: autostart; Flags: runhidden waituntilterminated
Filename: "{app}\opencharts.exe"; Parameters: "-open"; Description: "{cm:RunOpen}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\opencharts.exe"; Parameters: "setup uninstall"; Flags: runhidden waituntilterminated; RunOnceId: "OpenChartsSetupUninstall"

[Code]
procedure StopApp;
var
  rc: Integer;
begin
  { The program lives in the tray with no window: ask Windows to end it. }
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/IM opencharts.exe /F', '', SW_HIDE, ewWaitUntilTerminated, rc);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopApp;
  Result := '';
end;

function InitializeUninstall: Boolean;
begin
  StopApp;
  Result := True;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  data: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    data := ExpandConstant('{localappdata}\OpenCharts');
    if DirExists(data) then
      if MsgBox(FmtMessage(CustomMessage('AskDeleteData'), [data]), mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
        DelTree(data, True, True, True);
  end;
end;
