//go:generate go run asmtest/asm.go -out asmtest/add.s -stubs asmtest/stub.go

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	server "github.com/alex0x08/ungoogled-go/server"
	systray "github.com/alex0x08/ungoogled-go/systray"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/ui/wm"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

// Main functions, here will be the start point
// @author alex0x08

var (
	tray       *systray.TrayIcon
	httpSrv    *http.Server
	mainWindow *MyWindow
	//You can only set string variables with -X linker flag. From the docs:
	DebugMode = "true"
)

// This struct represents our main window.
type MyWindow struct {
	wnd     ui.WindowMain
	lblName ui.Static
	txtName ui.Edit
	btnShow ui.Button
}

// appends to UI log
func appendToLog(message string) {
	// could be no window yet
	if mainWindow == nil || mainWindow.txtName == nil {
		fmt.Println(message)
		return
	}
	// window could be not visible yet
	// and attempt to add message will raise an exception
	if !mainWindow.txtName.Hwnd().IsWindowVisible() {
		fmt.Println(message)
		return
	}
	// get current text
	txt := mainWindow.txtName.Text()
	// to avoid overflow
	if len(txt) > 512 {
		txt = ""
	}
	b := strings.Builder{}
	b.WriteString(txt)     // append existing text
	b.WriteString(message) // append new message
	b.WriteString("\r\n")  // this is Windows, so \r\n, not \n !
	// and finally set updated text (yep, there is no append, sorry)
	mainWindow.txtName.SetText(b.String())
}

func newMyWindow() *MyWindow {
	opts := ui.WindowMainOpts().
		ClassStyles(co.CS_NOCLOSE).
		Title("Tiny Server").
		ClientArea(win.SIZE{Cx: 600, Cy: 245})

	if DebugMode == "false" {
		// ID of icon resource, see resources folder
		// does not work in debug mode
		opts = opts.IconId(101)
	}
	// create main window
	wnd := ui.NewWindowMain(opts)
	// build UI
	me := &MyWindow{
		wnd: wnd,
		// add label
		lblName: ui.NewStatic(wnd,
			ui.StaticOpts().
				Text("Server log").
				Position(win.POINT{X: 10, Y: 22}),
		),
		// add shutdown button
		btnShow: ui.NewButton(wnd,
			ui.ButtonOpts().
				Text("&Quit").
				Position(win.POINT{X: 510, Y: 17}),
		),
		// add message log (text area)
		txtName: ui.NewEdit(wnd,
			ui.EditOpts().
				WndStyles(co.WS_CHILD|co.WS_VISIBLE|co.WS_VSCROLL).
				CtrlStyles(co.ES_AUTOHSCROLL|co.ES_MULTILINE|co.ES_LEFT|co.ES_READONLY).
				Position(win.POINT{X: 0, Y: 45}).
				Size(win.SIZE{Cx: 600, Cy: 200}),
		),
	}
	// setup handler on 'shutdown' button click
	me.btnShow.On().BnClicked(func() {
		// start confirmation dialog
		resp := me.wnd.Hwnd().MessageBox("Quit application?", "Confirm quit", co.MB_YESNO)
		// if user clicked 'YES' - shutdown application
		if resp == co.ID_YES {
			appendToLog("Exiting..")
			if httpSrv != nil {
				if err := httpSrv.Close(); err != nil {
					fmt.Printf("HTTP close error: %v", err)
				}
			}
			me.wnd.Hwnd().DestroyWindow()
			os.Exit(0)
		}
	})
	return me
}

func main() {
	// this is required for proper synchronization with UI thread
	runtime.LockOSThread()

	mainWindow = newMyWindow() // instantiate main window

	var trayIcon win.HICON

	// Load icon
	// in debug mode, there are no resources available, so we need to load
	// icons from FS
	if DebugMode == "false" {
		trayIcon = win.HICON(
			win.GetModuleHandle(win.StrOptNone()).LoadImage(
				win.ResIdInt(101),
				co.IMAGE_ICON,
				16, 16,
				co.LR_DEFAULTCOLOR,
			))

	} else {
		trayIcon = win.HICON(
			win.GetModuleHandle(win.StrOptNone()).LoadImage(
				win.ResIdStr("gopher.ico"),
				co.IMAGE_ICON,
				16, 16,
				co.LR_DEFAULTCOLOR|co.LR_LOADFROMFILE,
			))
	}

	// close systray on main window destroy
	mainWindow.wnd.On().WmDestroy(func() {
		if tray != nil {
			tray.Dispose()
		}
	})

	var configured = false // check for action that runs only once

	mainWindow.wnd.On().WmActivate(func(p wm.Activate) {
		// we need to run our handler logic only once at start
		if configured {
			return
		}
		configured = true
		go startServer()
	})
	// action on windows create
	// runs once
	mainWindow.wnd.On().WmNcCreate(func(p wm.Create) bool {
		// create systray
		tray := systray.CreateSysTray()
		// set handler on icon click - just focus on main window
		systray.SetTrayClickHandler(func() {
			systray.ShowWindow(uintptr(mainWindow.wnd.Hwnd()), systray.SW_SHOWNORMAL)
		})

		tray.SetIcon(uintptr(trayIcon))
		tray.SetTooltip("Tiny Server: click me to focus.")

		return true
	})

	mainWindow.wnd.RunAsMain() // ...and run
}

// starts HTTP server
func startServer() {
	appendToLog(fmt.Sprintf("Starting, debug mode: %s", DebugMode))

	// firewall bypass does not work correctly in debug mode
	if DebugMode == "false" {
		server.AddAppFirewallRule()
		appendToLog("Added firewall rule..")
	}
	// create request multiplexer, see https://pkg.go.dev/net/http#ServeMux
	mux := http.NewServeMux()
	// test assembler method
	mux.HandleFunc("/asmtest", server.TestAsmMethod)
	// upload & set wallpaper image
	mux.HandleFunc("/upload", server.UploadHandler)
	// default handler
	mux.HandleFunc("/", server.IndexHandler)

	// if this is production mode - bind to all interfaces
	if DebugMode == "false" {
		httpSrv = &http.Server{
			Addr:    ":8090",
			Handler: mux,
		}
	} else {
		// otherwise - bind to localhost (firewall bypass does not work in debug mode)
		httpSrv = &http.Server{
			Addr:    "localhost:8090",
			Handler: mux,
		}
	}

	appendToLog(fmt.Sprintf("Server started at %s", httpSrv.Addr))
	// set logging handler
	server.SetMessageLogHandler(appendToLog)
	httpSrv.ListenAndServe() // here will be lock
}
