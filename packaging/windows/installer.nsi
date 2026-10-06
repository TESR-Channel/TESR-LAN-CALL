; TESR LAN Call - Windows installer (NSIS 3)
; build: makensis -DVERSION=2.0.0 -DSRC=<dir with exe> -DOUT=<setup.exe> installer.nsi
Unicode true
!include "MUI2.nsh"
!include "Sections.nsh"

!ifndef VERSION
  !define VERSION "2.0.0"
!endif
!define APPNAME "TESR LAN Call"
!define EXE "TESR-LAN-Call.exe"
!define UNKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\TESRLanCall"
!define RUNKEY "Software\Microsoft\Windows\CurrentVersion\Run"

Name "${APPNAME}"
OutFile "${OUT}"
InstallDir "$PROGRAMFILES64\TESR LAN Call"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "TESR Co., Ltd."
VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APPNAME}"
VIAddVersionKey "CompanyName" "TESR Co., Ltd."
VIAddVersionKey "FileDescription" "${APPNAME} Setup"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "TESR Co., Ltd."

!define MUI_ICON "${SRC}/icon.ico"
!define MUI_UNICON "${SRC}/icon.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "${SRC}/installer-side.bmp"
!define MUI_UNWELCOMEFINISHPAGE_BITMAP "${SRC}/installer-side.bmp"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_BITMAP "${SRC}/installer-header.bmp"
!define MUI_WELCOMEPAGE_TITLE "ติดตั้ง TESR LAN Call"
!define MUI_WELCOMEPAGE_TEXT "โปรแกรมโทรวิดีโอภายในองค์กรผ่านวง LAN$\r$\n$\r$\nกด ถัดไป เพื่อติดตั้ง เสร็จแล้วจะมีไอคอนบนหน้าจอ Desktop ให้ทันที"
!define MUI_COMPONENTSPAGE_SMALLDESC
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "เปิด TESR LAN Call ตอนนี้"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "Thai"

Section "TESR LAN Call (จำเป็น)" SecMain
  SectionIn RO
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  Sleep 500
  SetOutPath "$INSTDIR"
  File "${SRC}/${EXE}"
  File "${SRC}/icon.ico"
  SetShellVarContext all
  CreateShortCut "$DESKTOP\TESR LAN Call.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\icon.ico" 0
  CreateDirectory "$SMPROGRAMS\TESR LAN Call"
  CreateShortCut "$SMPROGRAMS\TESR LAN Call\TESR LAN Call.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\icon.ico" 0
  CreateShortCut "$SMPROGRAMS\TESR LAN Call\ถอนการติดตั้ง TESR LAN Call.lnk" "$INSTDIR\uninstall.exe"
  ; เปิดไฟร์วอลล์ให้โปรแกรม ผู้ใช้ไม่ต้องกด Allow
  nsExec::Exec 'netsh advfirewall firewall delete rule name="TESR LAN Call"'
  nsExec::Exec 'netsh advfirewall firewall add rule name="TESR LAN Call" dir=in action=allow program="$INSTDIR\${EXE}" enable=yes profile=any'
  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "${UNKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKLM "${UNKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNKEY}" "Publisher" "TESR Co., Ltd."
  WriteRegStr HKLM "${UNKEY}" "DisplayIcon" "$INSTDIR\icon.ico"
  WriteRegStr HKLM "${UNKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "${UNKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNKEY}" "NoRepair" 1
SectionEnd

Section "เปิดเองตอนเปิดเครื่อง (แนะนำ: รับสายได้ตลอด)" SecAuto
  WriteRegStr HKCU "${RUNKEY}" "TESR LAN Call" '"$INSTDIR\${EXE}" --background'
SectionEnd

Section /o "โหมดหุ่นยนต์ / จอแสดงผล (เต็มจอ + รับสายอัตโนมัติ)" SecRobot
  WriteRegStr HKCU "${RUNKEY}" "TESR LAN Call" '"$INSTDIR\${EXE}" --robot'
SectionEnd

LangString DESC_Main ${LANG_THAI} "ตัวโปรแกรม ไอคอนบน Desktop และเมนู Start"
LangString DESC_Auto ${LANG_THAI} "ทำงานเบื้องหลังตั้งแต่เปิดเครื่อง เมื่อมีสายเข้าหน้าต่างจะเด้งขึ้นมาเอง"
LangString DESC_Robot ${LANG_THAI} "สำหรับเครื่องบนหุ่น Kuro-X หรือจอหน้างาน: เปิดเต็มจอ และรับทุกสายทันทีโดยไม่ต้องมีคนกด"
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} $(DESC_Main)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAuto} $(DESC_Auto)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRobot} $(DESC_Robot)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  Sleep 500
  SetShellVarContext all
  Delete "$DESKTOP\TESR LAN Call.lnk"
  RMDir /r "$SMPROGRAMS\TESR LAN Call"
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\icon.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegValue HKCU "${RUNKEY}" "TESR LAN Call"
  DeleteRegKey HKLM "${UNKEY}"
  nsExec::Exec 'netsh advfirewall firewall delete rule name="TESR LAN Call"'
SectionEnd
