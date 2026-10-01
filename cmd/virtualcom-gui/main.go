//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/lxn/walk"
	virtualcom "github.com/xiaolengWangWang/virtualcom"
	"github.com/xiaolengWangWang/virtualcom/gui"
	"github.com/xiaolengWangWang/virtualcom/internal/vcom"
)

func parsePresetPairs(args []string) ([][2]string, error) {
	var pairs [][2]string
	for i := 0; i < len(args); i++ {
		if args[i] != "--pair" {
			return nil, fmt.Errorf("未知参数 %s；请使用 --pair COM10:COM11", args[i])
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("--pair 后需要 COM10:COM11")
		}
		i++
		parts := strings.Split(args[i], ":")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("串口对格式应为 COM10:COM11")
		}
		pairs = append(pairs, [2]string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
	}
	return pairs, nil
}

func main() {
	mgr := virtualcom.NewManager()
	defer mgr.CloseAll()
	presets, err := parsePresetPairs(os.Args[1:])
	if err != nil {
		walk.MsgBox(nil, "启动参数", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
		return
	}
	if _, err := vcom.RemoveStaleLinks(); err != nil {
		walk.MsgBox(nil, "清理失效端口", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconWarning)
	}
	for _, pair := range presets {
		if _, err := mgr.Create(pair[0], pair[1]); err != nil {
			walk.MsgBox(nil, "创建串口对", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
			return
		}
	}
	w, _, err := gui.NewWindow(nil, mgr, nil)
	if err != nil {
		walk.MsgBox(nil, "VirtualCOM", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
		return
	}
	w.Show()
	w.Run()
}
