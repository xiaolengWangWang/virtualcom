//go:build windows

package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	virtualcom "github.com/xiaolengWangWang/virtualcom"
)

func TestPageManagesPairsAndPreservesThemOnClose(t *testing.T) {
	if os.Getenv("VIRTUALCOM_GUI_TEST") != "1" {
		t.Skip("set VIRTUALCOM_GUI_TEST=1 for native controls")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	manifest, err := filepath.Abs("../cmd/virtualcom-gui/VirtualCOM.exe.manifest")
	if err != nil {
		t.Fatal(err)
	}
	ctx := win.ACTCTX{Source: syscall.StringToUTF16Ptr(manifest)}
	h := win.CreateActCtx(&ctx)
	if h == win.HANDLE(^uintptr(0)) {
		t.Fatal("CreateActCtx failed")
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	defer kernel.NewProc("ReleaseActCtx").Call(uintptr(h))
	cookie, ok := win.ActivateActCtx(h)
	if !ok {
		t.Fatal("ActivateActCtx failed")
	}
	defer kernel.NewProc("DeactivateActCtx").Call(0, cookie)
	mgr := virtualcom.NewManager()
	defer mgr.CloseAll()
	w, page, err := NewWindow(nil, mgr, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !w.IsDisposed() {
			w.SetSuspended(true)
			time.Sleep(200 * time.Millisecond)
			w.Dispose()
		}
	}()
	if page.toggle.Enabled() || page.remove.Enabled() {
		t.Fatal("actions enabled without a selection")
	}
	page.portA.SetText("invalid")
	page.portB.SetText("COM10")
	page.createPair()
	if len(mgr.List().Pairs) != 0 || page.feedback.Text() == "" {
		t.Fatal("invalid input created a pair or gave no feedback")
	}
	page.portA.SetText("")
	page.portB.SetText("")
	page.createPair()
	pairs := mgr.List().Pairs
	if len(pairs) != 1 {
		t.Fatalf("created %d pairs", len(pairs))
	}
	id := pairs[0].ID
	if !page.toggle.Enabled() || !page.remove.Enabled() {
		t.Fatal("new pair was not selected")
	}
	page.togglePair()
	if mgr.List().Pairs[0].Enabled {
		t.Fatal("disable did not reach manager")
	}
	page.Refresh()
	if page.selectedID() != id {
		t.Fatal("refresh lost selection")
	}
	page.togglePair()
	if !mgr.List().Pairs[0].Enabled {
		t.Fatal("enable did not reach manager")
	}
	w.SetSuspended(true)
	time.Sleep(200 * time.Millisecond)
	w.Dispose()
	if len(mgr.List().Pairs) != 1 {
		t.Fatal("closing management window destroyed application-owned ports")
	}
	client, err := virtualcom.OpenPort(pairs[0].A.Name)
	if err != nil {
		t.Fatalf("port unusable after window closes: %v", err)
	}
	client.Close()
}

func TestTableShowsDisabledAndBusyPorts(t *testing.T) {
	m := &pairModel{pairs: []virtualcom.PairInfo{{ID: "a", A: virtualcom.PortInfo{Name: "COM10"}, B: virtualcom.PortInfo{Name: "COM11"}}}}
	if m.Value(0, 2) != "已停用" {
		t.Fatalf("disabled state: %v", m.Value(0, 2))
	}
	m.pairs[0].Enabled = true
	m.pairs[0].A.State = virtualcom.StateInUse
	if m.Value(0, 2) != "通信中" {
		t.Fatalf("busy state: %v", m.Value(0, 2))
	}
	var _ walk.TableModel = m
}
