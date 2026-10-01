//go:build windows

package gui

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/image/draw"
)

// Embedded PNGs keep the page independent of disk caches and the host's resource IDs.
//
//go:embed assets/*.png
var iconAssets embed.FS

func (p *Page) icon(name string) *walk.Icon {
	dpi := 96
	if dc := win.GetDC(0); dc != 0 {
		if value := int(win.GetDeviceCaps(dc, win.LOGPIXELSX)); value > 0 {
			dpi = value
		}
		win.ReleaseDC(0, dc)
	}
	if p.window != nil {
		dpi = p.window.DPI()
	}
	key := fmt.Sprintf("%s@%d", name, dpi)
	if icon := p.icons[key]; icon != nil {
		return icon
	}
	data, err := iconAssets.ReadFile("assets/" + name + ".png")
	if err != nil {
		return nil
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	// Walk's in-memory icons only have a handle at their creation DPI. Create
	// the native-size image for this window, and rebind when its DPI changes.
	logicalSize := 18
	if name == "app" {
		logicalSize = 32
	}
	size := (logicalSize*dpi + 48) / 96
	scaled := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), im, im.Bounds(), draw.Src, nil)
	icon, err := walk.NewIconFromImageForDPI(scaled, dpi)
	if err != nil {
		return nil
	}
	p.icons[key] = icon
	return icon
}

func (p *Page) applyIcons() {
	if p.iconDPI == p.window.DPI() {
		return
	}
	p.iconDPI = p.window.DPI()
	p.window.SetIcon(p.icon("app"))
	p.brand.SetImage(p.icon("app"))
	p.create.SetImage(p.icon("create"))
	p.remove.SetImage(p.icon("delete"))
	p.refresh.SetImage(p.icon("refresh"))
	p.diagnostics.SetImage(p.icon("diagnostics"))
	p.updateActions()
}
