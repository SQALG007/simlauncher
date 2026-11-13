; Installeur Windows de RigStart (Inno Setup 6).
; Compilé par GitHub Actions : iscc /DAppVersion=1.8.1 /DExeDir=..\dist installer\rigstart.iss
; Installation par utilisateur, sans droits administrateur, pour que les mises à jour
; intégrées puissent remplacer l'exe.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef ExeDir
  #define ExeDir "..\dist"
#endif

[Setup]
AppId={{6F1C2B7E-4A3D-4E8B-9C21-5D7A0B3E9F14}
AppName=RigStart
AppVersion={#AppVersion}
AppVerName=RigStart {#AppVersion}
AppPublisher=RigStart contributors
AppPublisherURL=https://github.com/rigstart/rigstart
AppSupportURL=https://github.com/rigstart/rigstart/issues
AppUpdatesURL=https://github.com/rigstart/rigstart/releases
DefaultDirName={localappdata}\Programs\RigStart
DefaultGroupName=RigStart
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=RigStart-Setup
SetupIconFile=..\ui\icon.ico
UninstallDisplayIcon={app}\RigStart.exe
UninstallDisplayName=RigStart
LicenseFile=..\LICENSE
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=force
RestartApplications=no
ShowLanguageDialog=auto

[Languages]
Name: "en"; MessagesFile: "compiler:Default.isl"
Name: "fr"; MessagesFile: "compiler:Languages\French.isl"
Name: "pt"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "es"; MessagesFile: "compiler:Languages\Spanish.isl"

[CustomMessages]
en.AutoStart=Start RigStart with Windows (icon next to the clock)
fr.AutoStart=Démarrer RigStart avec Windows (icône près de l'horloge)
pt.AutoStart=Iniciar o RigStart com o Windows (ícone ao lado do relógio)
es.AutoStart=Iniciar RigStart con Windows (icono junto al reloj)
en.DeleteConfig=Also delete your RigStart settings, play time and pictures?%n%nChoose No to keep them for a later reinstall.
fr.DeleteConfig=Supprimer aussi tes réglages, heures de jeu et images RigStart ?%n%nChoisis Non pour les garder en vue d'une réinstallation.
pt.DeleteConfig=Excluir também suas configurações, tempo de jogo e imagens do RigStart?%n%nEscolha Não para mantê-los para uma reinstalação.
es.DeleteConfig=¿Borrar también tus ajustes, horas de juego e imágenes de RigStart?%n%nElige No para conservarlos para una reinstalación.

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "{cm:AutoStart}"; Flags: unchecked

[Files]
Source: "{#ExeDir}\RigStart.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\RigStart"; Filename: "{app}\RigStart.exe"
Name: "{autodesktop}\RigStart"; Filename: "{app}\RigStart.exe"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "RigStart"; ValueData: """{app}\RigStart.exe"" --tray"; Tasks: autostart

[Run]
Filename: "{app}\RigStart.exe"; Description: "{cm:LaunchProgram,RigStart}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM RigStart.exe"; Flags: runhidden; RunOnceId: "KillRigStart"

[UninstallDelete]
Type: files; Name: "{app}\RigStart.exe.old*"

[Code]
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
  begin
    { L'application peut avoir créé elle-même l'entrée « Démarrer avec Windows ». }
    RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'RigStart');
    if DirExists(ExpandConstant('{userappdata}\RigStart')) and not UninstallSilent then
      if MsgBox(CustomMessage('DeleteConfig'), mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
        DelTree(ExpandConstant('{userappdata}\RigStart'), True, True, True);
  end;
end;
