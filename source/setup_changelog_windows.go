//go:build windows

package main

import (
    "os"
    "path/filepath"
    "strings"
    "syscall"
    "unsafe"
)

// An owned window in Setup's message loop. No browser, temporary HTML or editor.
var changelogWindow uintptr
var changelogProcedure = syscall.NewCallback(changelogWindowProc)

func changelogWindowProc(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
    g := activeSetupGUI
    if g == nil { result, _, _ := guiUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam); return result }
    switch message {
    case guiWMCommand:
        if int(wparam&0xffff) != guiIDSave { break }
        fallthrough
    case guiWMClose:
        if err := os.MkdirAll(appDir(g.dir), 0755); err == nil {
            _ = os.WriteFile(filepath.Join(appDir(g.dir), "changelog-seen.txt"), []byte(bridgeVersion+"\n"), 0644)
        }
        guiUser.NewProc("DestroyWindow").Call(window)
        changelogWindow = 0
        guiUser.NewProc("SetForegroundWindow").Call(g.window)
        return 0
    case guiWMColorEdit, guiWMColorStatic, guiWMColorButton, guiWMDrawItem:
        return setupWindowProcedure(window, message, wparam, lparam)
    }
    result, _, _ := guiUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
    return result
}

func (g *setupGUI) showReleaseChangelog(force bool) uintptr {
    if changelogWindow != 0 { return changelogWindow }
    if !force {
        if bridgeVersion != "2.2.2" { return 0 }
        seen, _ := os.ReadFile(filepath.Join(appDir(g.dir), "changelog-seen.txt"))
        if strings.TrimSpace(string(seen)) == bridgeVersion { return 0 }
    }
    instance, _, _ := setupKernel.NewProc("GetModuleHandleW").Call(0)
    cursor, _, _ := guiUser.NewProc("LoadCursorW").Call(0, 32512)
    class := guiWindowClass{Procedure:changelogProcedure,Instance:instance,Icon:g.icon,SmallIcon:g.smallIcon,Cursor:cursor,Background:g.background,Name:winPtr("OpenOMSIReleaseNotes")}
    class.Size=uint32(unsafe.Sizeof(class))
    guiUser.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
    style:=uintptr(0x00C00000|0x00080000|0x02000000)
    rect:=guiRect{Right:720,Bottom:650}
    guiUser.NewProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&rect)),style,0,0)
    var parent guiRect
    guiUser.NewProc("GetWindowRect").Call(g.window,uintptr(unsafe.Pointer(&parent)))
    width,height:=int(rect.Right-rect.Left),int(rect.Bottom-rect.Top)
    x,y:=int(parent.Left)+(int(parent.Right-parent.Left)-width)/2,int(parent.Top)+20
    if x<0{x=0};if y<0{y=0}
    window,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(class.Name)),uintptr(unsafe.Pointer(winPtr("OpenOMSI BCS Bridge — 2.2.2"))),style,uintptr(x),uintptr(y),uintptr(width),uintptr(height),g.window,0,instance,0)
    if window==0{return 0};changelogWindow=window
    control:=func(kind,text string,x,y,w,h int,style uintptr,id int,font uintptr)uintptr{
        c,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(winPtr(kind))),uintptr(unsafe.Pointer(winPtr(text))),guiWSChild|guiWSVisible|style,uintptr(x),uintptr(y),uintptr(w),uintptr(h),window,uintptr(id),instance,0)
        guiUser.NewProc("SendMessageW").Call(c,guiWMSetFont,font,1);return c
    }
    tr:=func(pt,en,de string)string{return localText(g.config.Language,pt,en,de)}
    control("STATIC",tr("Atualização instalada","Update installed","Update installiert"),32,24,650,24,0,0,g.smallFont)
    control("STATIC",tr("Novidades da 2.2.2","What's new in 2.2.2","Neu in 2.2.2"),32,52,650,38,0,0,g.titleFont)
    rows:=[][2]string{
        {tr("Eventos do BCS ganham vida","BCS events come to life","BBS-Ereignisse werden sichtbar"),tr("Ocorrências, equipes de emergência e atendimento no cenário.","Incidents, emergency crews and responses in the scene.","Vorfälle, Einsatzfahrzeuge und Hilfe in der Spielwelt.")},
        {tr("Inspetor a bordo","Inspector on board","Kontrolleur an Bord"),tr("Um personagem acompanha a viagem e a fiscalização do BCS.","A character follows the trip and BCS ticket inspection.","Eine Figur begleitet die Fahrt und die BBS-Fahrkartenkontrolle.")},
        {tr("Troca de motorista corrigida","Driver handover fixes","Fahrerwechsel korrigiert"),tr("GPS continua do ponto atual; som e passageiros mantêm seu estado.","The GPS keeps its progress; audio and seated riders retain their state.","GPS-Fortschritt, Ton und sitzende Fahrgäste bleiben erhalten.")},
        {tr("Embarque mais flexível","More flexible boarding","Flexiblerer Einstieg"),tr("Com a porta aberta, passageiros próximos caminham até o ônibus.","Nearby passengers walk to a stopped bus with its door open.","Nahe Fahrgäste gehen zum stehenden Bus mit geöffneter Tür.")},
        {tr("Tráfego mais equilibrado","More balanced traffic","Ausgewogenerer Verkehr"),tr("Carros distribuídos pelas faixas e nos dois sentidos.","Vehicles spread across lanes and both directions.","Fahrzeuge verteilen sich auf Fahrstreifen und beide Richtungen.")},
        {tr("Passageiros mais participativos","More passenger reactions","Mehr Fahrgastreaktionen"),tr("Agradecimentos e pedidos de abertura das portas com vozes existentes.","Thanks and door-opening requests use the installed voices.","Dank und Türöffnungswünsche mit vorhandenen Stimmen.")},
        {tr("Jogo e servidor 0.3.46","Game and server 0.3.46","Spiel und Server 0.3.46"),tr("A nova base oficial junto das melhorias da Bridge.","The new official base with the Bridge improvements.","Die neue offizielle Basis zusammen mit den Bridge-Verbesserungen.")},
    }
    for i,row:=range rows{top:=110+i*62;control("STATIC",row[0],32,top,650,24,0,0,g.font);control("STATIC",row[1],32,top+25,650,30,0,0,g.smallFont)}
    control("BUTTON",tr("Vamos jogar","Let's play","Los geht's"),476,590,210,40,guiWSTabStop|0xB,guiIDSave,g.font)
    guiUser.NewProc("ShowWindow").Call(window,1);guiUser.NewProc("UpdateWindow").Call(window)
    return window
}
