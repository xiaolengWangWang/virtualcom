//go:build windows

// Package gui provides the same native port-management page for standalone
// VirtualCOM and applications that embed its Manager.
package gui

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	virtualcom "github.com/xiaolengWangWang/virtualcom"
)

type Page struct {
	iconDPI                                 int
	brand                                   *walk.ImageView
	create, refresh, diagnostics, copyPorts *walk.PushButton
	icons                                   map[string]*walk.Icon
	window                                  *walk.MainWindow
	manager                                 *virtualcom.Manager
	model                                   *pairModel
	portA, portB                            *walk.LineEdit
	table                                   *walk.TableView
	toggle, remove                          *walk.PushButton
	summary, feedback                       *walk.Label
	selection                               *walk.Label
	changed                                 func()
	closed                                  atomic.Bool
	done                                    chan struct{}
}

// NewWindow creates a management window. The caller owns manager: closing this
// window never closes its ports. changed runs on the UI thread after mutations.
func NewWindow(owner walk.Form, manager *virtualcom.Manager, changed func()) (*walk.MainWindow, *Page, error) {
	if manager == nil {
		return nil, nil, fmt.Errorf("virtualcom: nil manager")
	}
	p := &Page{manager: manager, model: &pairModel{}, changed: changed, done: make(chan struct{}), icons: make(map[string]*walk.Icon)}
	created := false
	defer func() {
		if !created {
			for _, icon := range p.icons {
				icon.Dispose()
			}
		}
	}()
	blue := walk.RGB(23, 60, 101)
	muted := walk.RGB(92, 105, 124)
	white := walk.RGB(255, 255, 255)
	if err := (MainWindow{
		AssignTo: &p.window,
		Title:    "VirtualCOM · 虚拟串口管理",
		Size:     Size{Width: 860, Height: 560}, MinSize: Size{Width: 680, Height: 480},
		Font:       Font{Family: "Microsoft YaHei UI", PointSize: 9},
		Background: SolidColorBrush{Color: walk.RGB(244, 247, 251)},
		Layout:     VBox{Margins: Margins{Left: 20, Top: 18, Right: 20, Bottom: 14}, Spacing: 12},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
				ImageView{AssignTo: &p.brand, Image: p.icon("app"), Mode: ImageViewModeIdeal, MinSize: Size{Width: 32, Height: 32}, MaxSize: Size{Width: 32, Height: 32}},
				Label{Text: "虚拟串口", Font: Font{Family: "Microsoft YaHei UI", PointSize: 19, Bold: true}, TextColor: blue},
				HSpacer{}, Label{Text: "VirtualCOM " + virtualcom.Version, TextColor: muted},
			}},
			Label{Text: "连接两个端口，让应用双向收发数据。", TextColor: muted},
			Composite{Background: SolidColorBrush{Color: white}, Layout: VBox{Margins: Margins{Left: 14, Top: 12, Right: 14, Bottom: 12}, Spacing: 8}, Children: []Widget{
				Label{Text: "新建串口对", Font: Font{Family: "Microsoft YaHei UI", PointSize: 10, Bold: true}, TextColor: blue},
				Composite{Layout: HBox{MarginsZero: true, Spacing: 10}, Children: []Widget{
					Label{Text: "端口 A"}, LineEdit{AssignTo: &p.portA, CueBanner: "自动分配", MinSize: Size{Width: 110}, StretchFactor: 1},
					Label{Text: "⇄", TextColor: blue},
					Label{Text: "端口 B"}, LineEdit{AssignTo: &p.portB, CueBanner: "自动分配", MinSize: Size{Width: 110}, StretchFactor: 1},
					PushButton{AssignTo: &p.create, Text: "创建串口对", Image: p.icon("create"), MinSize: Size{Width: 126, Height: 30}, OnClicked: p.createPair},
				}},
				Label{Text: "留空自动分配；也可输入 COM10、COM11 等端口名。", TextColor: muted},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "串口对", Font: Font{Family: "Microsoft YaHei UI", PointSize: 10, Bold: true}}, HSpacer{},
				Label{AssignTo: &p.summary, TextColor: muted},
			}},
			TableView{AssignTo: &p.table, Model: p.model, StretchFactor: 1, MinSize: Size{Height: 130},
				AlternatingRowBG: true, LastColumnStretched: true, ColumnsOrderable: false,
				OnCurrentIndexChanged: p.updateActions,
				Columns: []TableViewColumn{
					{Title: "端口 A", Width: 85}, {Title: "端口 B", Width: 85}, {Title: "状态", Width: 80},
					{Title: "占用程序", Width: 190}, {Title: "A → B", Width: 95}, {Title: "B → A", Width: 95},
				}},
			Composite{Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
				PushButton{AssignTo: &p.toggle, Text: "停用", Image: p.icon("pause"), Enabled: false, MinSize: Size{Width: 88, Height: 30}, OnClicked: p.togglePair},
				PushButton{AssignTo: &p.remove, Text: "删除", Image: p.icon("delete"), Enabled: false, MinSize: Size{Width: 88, Height: 30}, OnClicked: p.confirmDelete},
				PushButton{AssignTo: &p.copyPorts, Text: "复制端口", Image: p.icon("copy"), Enabled: false, MinSize: Size{Width: 100, Height: 30}, OnClicked: p.copySelectedPorts},
				HSpacer{}, PushButton{AssignTo: &p.refresh, Text: "刷新", Image: p.icon("refresh"), MinSize: Size{Width: 88, Height: 30}, OnClicked: p.Refresh},
				PushButton{AssignTo: &p.diagnostics, Text: "诊断", Image: p.icon("diagnostics"), MinSize: Size{Width: 88, Height: 30}, OnClicked: p.showDiagnostics},
			}},
			Label{AssignTo: &p.selection, Text: "选择串口对后，可启停、复制端口名或删除。", TextColor: muted, EllipsisMode: EllipsisEnd},
			Label{AssignTo: &p.feedback, Text: "暂无串口对，点击“创建串口对”开始。", TextColor: muted, EllipsisMode: EllipsisEnd},
			Label{Text: "兼容已适配的应用（如 CommBox）；波特率等串口参数不生效。\r\n关闭管理页不影响通信；退出创建端口的程序后，端口释放。", TextColor: muted},
		},
	}).Create(); err != nil {
		return nil, nil, err
	}
	if owner != nil {
		if err := p.window.SetOwner(owner); err != nil {
			p.window.Dispose()
			return nil, nil, err
		}
	}
	p.applyIcons()
	p.window.SizeChanged().Attach(p.applyIcons)
	p.window.Disposing().Attach(func() {
		if p.closed.CompareAndSwap(false, true) {
			close(p.done)
			for _, icon := range p.icons {
				icon.Dispose()
			}
		}
	})
	p.Refresh()
	go p.refreshLoop()
	created = true
	return p.window, p, nil
}

func (p *Page) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			if p.closed.Load() {
				return
			}
			p.window.Synchronize(func() {
				if !p.closed.Load() {
					p.Refresh()
				}
			})
		}
	}
}

func (p *Page) selectedID() string {
	if p.table == nil {
		return ""
	}
	i := p.table.CurrentIndex()
	if i < 0 || i >= len(p.model.pairs) {
		return ""
	}
	return p.model.pairs[i].ID
}

// Refresh retains the selected pair and avoids rebuilding unchanged rows.
func (p *Page) Refresh() {
	if p.closed.Load() {
		return
	}
	id := p.selectedID()
	pairs := p.manager.List().Pairs
	if !reflect.DeepEqual(p.model.pairs, pairs) {
		p.model.pairs = pairs
		p.model.PublishRowsReset()
		p.selectPair(id)
	}
	enabled := 0
	for _, pair := range pairs {
		if pair.Enabled {
			enabled++
		}
	}
	p.summary.SetText(fmt.Sprintf("%d 对 · %d 对已启用", len(pairs), enabled))
	p.updateActions()
}

func (p *Page) selectPair(id string) {
	for i, pair := range p.model.pairs {
		if pair.ID == id {
			p.table.SetCurrentIndex(i)
			return
		}
	}
	p.table.SetCurrentIndex(-1)
}

func (p *Page) updateActions() {
	if p.toggle == nil || p.remove == nil {
		return
	}
	i := p.table.CurrentIndex()
	selected := i >= 0 && i < len(p.model.pairs)
	p.toggle.SetEnabled(selected)
	p.remove.SetEnabled(selected)
	if p.copyPorts != nil {
		p.copyPorts.SetEnabled(selected)
	}
	if p.selection != nil {
		text := "选择串口对后，可启停、复制端口名或删除。"
		if selected {
			pair := p.model.pairs[i]
			text = fmt.Sprintf("已选择  %s  ⇄  %s", pair.A.Name, pair.B.Name)
		}
		p.selection.SetText(text)
	}
	label := "停用"
	iconName := "pause"
	if selected && !p.model.pairs[i].Enabled {
		label = "启用"
		iconName = "play"
	}
	p.toggle.SetText(label)
	if icon := p.icon(iconName); p.toggle.Image() != icon {
		p.toggle.SetImage(icon)
	}
}

func (p *Page) copySelectedPorts() {
	if p.selectedID() == "" {
		return
	}
	pair := p.model.pairs[p.table.CurrentIndex()]
	if err := walk.Clipboard().SetText(pair.A.Name + "\r\n" + pair.B.Name); err != nil {
		p.message("复制失败："+err.Error(), true)
		return
	}
	p.message("已复制两个端口名，每行一个。", false)
}

func (p *Page) message(text string, failed bool) {
	color := walk.RGB(48, 111, 78)
	if failed {
		color = walk.RGB(180, 48, 48)
	}
	p.feedback.SetTextColor(color)
	p.feedback.SetText(text)
	p.feedback.SetToolTipText(text)
}

func (p *Page) notify() {
	p.Refresh()
	if p.changed != nil {
		p.changed()
	}
}

func (p *Page) createPair() {
	pair, err := p.manager.Create(strings.ToUpper(strings.TrimSpace(p.portA.Text())), strings.ToUpper(strings.TrimSpace(p.portB.Text())))
	if err != nil {
		p.message(err.Error(), true)
		return
	}
	p.notify()
	p.selectPair(pair.ID)
	p.updateActions()
	p.message(fmt.Sprintf("已创建 %s ⇄ %s，可分别在两个应用中连接。", pair.A.Name, pair.B.Name), false)
}

func (p *Page) togglePair() {
	id := p.selectedID()
	if id == "" {
		return
	}
	on := !p.model.pairs[p.table.CurrentIndex()].Enabled
	if err := p.manager.SetEnabled(id, on); err != nil {
		p.message(err.Error(), true)
		return
	}
	p.notify()
	if on {
		p.message("串口对已启用。", false)
	} else {
		p.message("串口对已停用，编号配置已保留。", false)
	}
}

func (p *Page) confirmDelete() {
	id := p.selectedID()
	if id == "" {
		return
	}
	pair := p.model.pairs[p.table.CurrentIndex()]
	if walk.MsgBox(p.window, "删除串口对", fmt.Sprintf("删除 %s ⇄ %s？", pair.A.Name, pair.B.Name), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	if err := p.manager.Delete(id); err != nil {
		p.message(err.Error(), true)
		return
	}
	p.notify()
	p.message("串口对已删除。", false)
}

func (p *Page) showDiagnostics() {
	var dlg *walk.Dialog
	if err := (Dialog{AssignTo: &dlg, Title: "VirtualCOM 诊断", Size: Size{Width: 700, Height: 440}, MinSize: Size{Width: 500, Height: 300}, Layout: VBox{}, Children: []Widget{
		TextEdit{ReadOnly: true, VScroll: true, Text: p.manager.Diagnose(), StretchFactor: 1},
		PushButton{Text: "关闭", OnClicked: func() { dlg.Accept() }},
	}}).Create(p.window); err != nil {
		p.message(err.Error(), true)
		return
	}
	defer dlg.Dispose()
	dlg.Run()
}

type pairModel struct {
	walk.TableModelBase
	pairs []virtualcom.PairInfo
}

func (m *pairModel) RowCount() int { return len(m.pairs) }
func (m *pairModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.pairs) {
		return ""
	}
	p := m.pairs[row]
	switch col {
	case 0:
		return p.A.Name
	case 1:
		return p.B.Name
	case 2:
		if !p.Enabled {
			return "已停用"
		}
		if p.Error != "" || p.A.State == virtualcom.StateError || p.B.State == virtualcom.StateError {
			return "异常"
		}
		if p.A.State == virtualcom.StateInUse || p.B.State == virtualcom.StateInUse {
			return "通信中"
		}
		return "就绪"
	case 3:
		var owners []string
		for _, port := range []virtualcom.PortInfo{p.A, p.B} {
			if port.State == virtualcom.StateInUse {
				name := port.Process
				if name == "" {
					name = fmt.Sprintf("PID %d", port.PID)
				}
				owners = append(owners, port.Name+": "+name)
			}
		}
		if len(owners) == 0 {
			return "—"
		}
		return strings.Join(owners, " / ")
	case 4:
		return formatBytes(p.A.TX)
	case 5:
		return formatBytes(p.B.TX)
	}
	return ""
}

func formatBytes(n uint64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
