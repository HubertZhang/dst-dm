package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
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

type Config struct {
	RoomCode string
	SaveCode bool
	// 从直播开放平台获取的 accessKey
	AccessKey string

	// 从直播开放平台获取的 secret
	AccessToken string

	AppIdStr string
}

var savedConfig Config

//go:embed index.html
var index string

func main() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	basic.DefaultLoggerGenerator = func() *slog.Logger {
		return logger.With("module", "bianka")
	}
	d, err := os.ReadFile("config.json")
	if err == nil {
		json.Unmarshal(d, &savedConfig)
	}

	if savedConfig.SaveCode {
		index = strings.Replace(index, `var saveCode = false;`, `var saveCode = true;`, 1)
		index = strings.Replace(index, `var roomCode = "";`, `var roomCode = "`+savedConfig.RoomCode+`";`, 1)
		index = strings.Replace(index, `var access_key_id = "";`, `var access_key_id = "`+savedConfig.AccessKey+`";`, 1)
		index = strings.Replace(index, `var access_key_secret = "";`, `var access_key_secret = "`+savedConfig.AccessToken+`";`, 1)
		index = strings.Replace(index, `var app_id = "";`, `var app_id = "`+savedConfig.AppIdStr+`";`, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("饥荒弹幕机")
	w.SetSize(420, 380, webview.HintFixed)

	err = w.Bind("callback", func(object map[string]any) error {
		roomCode := object["room_code"].(string)
		accessKey := object["access_key_id"].(string)
		accessToken := object["access_key_secret"].(string)
		appId := object["app_id"].(string)

		savedConfig.AccessKey = accessKey
		savedConfig.AccessToken = accessToken
		savedConfig.AppIdStr = appId
		savedConfig.RoomCode = roomCode

		save := object["save_code"].(bool)

		if save {
			savedConfig.SaveCode = true
			d, err := json.Marshal(savedConfig)
			if err != nil {
				return err
			}
			err = os.WriteFile("config.json", d, 0644)
			if err != nil {
				return err
			}
		}
		go func() {
			err := startServer(ctx, savedConfig, func() {
				w.Dispatch(func() {
					w.Eval(`setState("error", "转发已终止")`)
				})
			})
			if err != nil {
				w.Dispatch(func() {
					w.Eval(`setState("error", "无法启动服务器")`)
				})
				return
			}
			w.Dispatch(func() {
				w.Eval(`setState("running")`)
			})
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
	w.Bind("openBilibili", func(req ...any) error {
		return browser.OpenURL("https://open-live.bilibili.com/open-manage")
	})
	w.SetHtml(index)
	w.Run()
}
