package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/browser"
	"github.com/vtb-link/bianka/basic"
	webview "github.com/webview/webview_go"
	"golang.org/x/exp/slog"
)

var logger *slog.Logger

var (
	flagPort = flag.Int("port", 9876, "监听端口")
)

var config struct {
	RoomCode string
	SaveCode bool
}

//go:embed index.html
var index string

func main() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	basic.DefaultLoggerGenerator = func() *slog.Logger {
		return logger.With("module", "bianka")
	}
	d, err := os.ReadFile("config.json")
	if err == nil {
		json.Unmarshal(d, &config)
	}

	if config.SaveCode {
		index = strings.Replace(index, `var roomCode = "";`, `var roomCode = "`+config.RoomCode+`";`, 1)
		index = strings.Replace(index, `var saveCode = false;`, `var saveCode = true;`, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("饥荒弹幕机")
	w.SetSize(420, 300, webview.HintFixed)

	err = w.Bind("callback", func(roomCode string, save bool) error {
		if save {
			config.RoomCode = roomCode
			config.SaveCode = true
			d, err := json.Marshal(config)
			if err != nil {
				return err
			}
			err = os.WriteFile("config.json", d, 0644)
			if err != nil {
				return err
			}
		} else {
			config.RoomCode = roomCode
		}
		fmt.Println(config.RoomCode)
		w.Eval(`setState("processing")`)
		go func() {
			err := startServer(ctx, config.RoomCode, func() {
				w.Eval(`setState("error", "转发已终止")`)
			})
			if err != nil {
				w.Eval(`setState("error", "转发已终止")`)
				return
			}
			w.Eval(`setState("running")`)
		}()
		return nil
	})
	if err != nil {
		panic(err)
	}
	w.Bind("stop", func(req ...any) error {
		cancel()
		return nil
	})
	w.Bind("openRoom", func(req ...any) error {
		return browser.OpenURL("https://link.bilibili.com/p/center/index#/my-room/start-live")
	})
	w.SetHtml(index)
	w.Run()
}
