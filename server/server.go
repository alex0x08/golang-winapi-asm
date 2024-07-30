package server

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	ungoogled "github.com/alex0x08/ungoogled-go/asmtest"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// There will be functions, related to HTTP server
// @author alex0x08

var (
	user32DLL            = windows.NewLazyDLL("user32.dll")
	procSystemParamInfo  = user32DLL.NewProc("SystemParametersInfoW")
	messageLogCallback   func(message string) // a callback for UI message log
	SPI_SETDESKWALLPAPER = 0x0014
	uiParamEmpty         = 0x0000
	SPIF_SENDCHANGE      = 0x02
)

// Compile templates on start of the application
//
//go:embed upload.html
var uploadTemplate string

//go:embed index.html
var indexTemplate string

// load all templates
var templates, _ = template.Must(template.New("upload.html").
	Parse(uploadTemplate)).
	New("index.html").
	Parse(indexTemplate)

// set UI message log handler
func SetMessageLogHandler(fn func(message string)) {
	messageLogCallback = fn
}

// adds firewall rule via WinAPI to bypass confirmation screen
func AddAppFirewallRule() error {
	session, err := wf.New(&wf.Options{
		Name:    "ungoogled session",
		Dynamic: false,
	})
	if err != nil {
		return err
	}
	defer session.Close()
	guid, _ := windows.GenerateGUID()
	execPath, _ := os.Executable()
	appID, _ := wf.AppID(execPath)
	err = session.AddRule(&wf.Rule{
		ID:     wf.RuleID(guid),
		Name:   "Ungoogled",
		Layer:  wf.LayerALEAuthRecvAcceptV4,
		Weight: 800,
		Conditions: []*wf.Match{
			{
				Field: wf.FieldALEAppID,
				Op:    wf.MatchTypeEqual,
				Value: appID,
			},
		},
		Action: wf.ActionPermit,
	})

	if err != nil {
		return err
	}
	return nil
}

// serves / with index.html
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	display(w, "index", nil)
}

// serves /upload with UI and file upload processing
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		display(w, "upload", nil)
	case "POST":
		uploadFile(w, r)
	}
}

// logs message with callback on UI
func logMessage(message string) {
	if messageLogCallback != nil {
		messageLogCallback(message)
	} else {
		fmt.Println(message)
	}
}

// Display the named template
func display(w http.ResponseWriter, page string, data interface{}) {
	templates.ExecuteTemplate(w, page+".html", data)
}

// Uploads image file and set it as Windows desktop wallpaper
func uploadFile(w http.ResponseWriter, r *http.Request) {
	// Maximum upload of 10 MB files
	r.ParseMultipartForm(10 << 20)

	// Get handler for filename, size and headers
	file, handler, err := r.FormFile("imageFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}

	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	dst, err := os.CreateTemp("", "tmp_*_"+handler.Filename)
	if err != nil {
		fmt.Println("Error :")
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	defer dst.Close()

	// Copy the uploaded file to the created file on the filesystem
	if _, err := io.Copy(dst, file); err != nil {
		fmt.Println("Error :")
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// build full path to image
	imagePath, err := windows.UTF16PtrFromString(dst.Name())
	// check for errors
	if err != nil {
		fmt.Println("Error :")
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// call WinAPI  to change wallpaper to just uploaded image
	_, _, err = procSystemParamInfo.Call(uintptr(SPI_SETDESKWALLPAPER),
		uintptr(uiParamEmpty),
		uintptr(unsafe.Pointer(imagePath)), uintptr(SPIF_SENDCHANGE))
	// check for errors, respond 500 if any
	if err, ok := err.(syscall.Errno); ok {
		if err != 0 {
			fmt.Println("Error :")
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	logMessage(fmt.Sprintf("Background changed to %s", dst.Name()))
}

// a test API method to call function with Assembler inside
func TestAsmMethod(w http.ResponseWriter, req *http.Request) {

	query := req.URL.Query()
	fmt.Println("GET params were:", query)

	param1, param2 := query.Get("param1"), query.Get("param2")

	int1, _ := strconv.ParseUint(param1, 10, 64)
	int2, _ := strconv.ParseUint(param2, 10, 64)
	fmt.Fprintf(w, "int1: %v int2: %v \n", int1, int2)

	// yep, check stub.go in asmtest
	out := ungoogled.Add(int1, int2)

	fmt.Fprintf(w, "result: %v \n", out)
	logMessage(fmt.Sprintf("Called asm method with params: %v , %v and result: %v", int1, int2, out))

}
