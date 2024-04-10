package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"github.com/vtb-link/bianka/proto"
	"golang.org/x/exp/slog"
	"hubertzhang.com/dst-dm/room"
)

var logger *slog.Logger

var (
	flagAccessKey   = flag.String("key", "", "从直播开放平台获取的 accessKey")
	flagAccessToken = flag.String("secret", "", "从直播开放平台获取的 secret")
	flagAppID       = flag.Int64("app-id", 0, "插件 app-id")
	flagRoomCode    = flag.String("room-code", "", "主播身份码")
	flagPort        = flag.Int("port", 9876, "监听端口")
)

type Room struct {
	Code            string
	StartResponse   *live.AppStartResponse
	WSClient        *basic.WsClient
	LastContactTime time.Time

	mu       *sync.RWMutex
	Messages chan *proto.Cmd
}

func (r *Room) Handler(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.LastContactTime = time.Now()
	// 业务逻辑
	t := time.NewTimer(3 * time.Second)

	msgs := make([]*proto.Cmd, 0, 10)
	select {
	case msg := <-r.Messages:
		msgs = append(msgs, msg)
	case <-t.C:
		break
	}
LOOP:
	for i := 0; i < 9; i++ {
		select {
		case msg := <-r.Messages:
			msgs = append(msgs, msg)
		default:
			break LOOP
		}
	}
	logger.Debug("Returned msgs to DST", slog.Int("count", len(msgs)))
	err := json.NewEncoder(w).Encode(msgs)
	if err != nil {
		logger.Warn("Error encoding messages", slog.Any("err", err))
	}
}

func (r *Room) Handle(wsClient *basic.WsClient, msg *proto.Message) error {
	// sdk提供了自动解析消息的方法，可以快速解析为对应的cmd和data
	// 具体的cmd 可以参考 proto/cmd.go
	cmd, data, err := proto.AutomaticParsingMessageCommand(msg.Payload())
	if err != nil {
		return err
	}

	// 你可以使用cmd进行switch
	switch cmd {
	case proto.CmdLiveOpenPlatformDanmu:
		if len(r.Messages) > 90 {
			for i := 0; i < 10; i++ {
				<-r.Messages
			}
		}
		r.Messages <- &proto.Cmd{
			Cmd:  cmd,
			Data: data.(*proto.CmdDanmuData),
		}
	}
	return nil
}

func main() {
	flag.Parse()

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *flagAccessKey == "" || *flagAccessToken == "" || *flagAppID == 0 || *flagRoomCode == "" {
		fmt.Println("AccessKey 或 Secret 或 AppID 缺失")
		fmt.Println()
		fmt.Println("使用方式：")
		fmt.Println("  start -key=xxx -secret=xxx -app-id=xxx -port=9876 -room-code=xxx")
		fmt.Println()
		fmt.Println("参数说明：")
		flag.PrintDefaults()
		return
	}
	sdkConfig := live.NewConfig(*flagAccessKey, *flagAccessToken, *flagAppID)

	// 创建sdk实例
	sdk := live.NewClient(sdkConfig)

	basic.DefaultLoggerGenerator = func() *slog.Logger {
		return logger.With("module", "bianka")
	}

	resp, err := sdk.AppStart(*flagRoomCode)
	if err != nil {
		logger.Error("服务启动失败", slog.Any("err", err))
		return
	}

	rm, err := room.New(*flagRoomCode, resp, func(wcs *basic.WsClient, _ basic.StartResp, closeType int) {
		canContinue := false
		switch closeType {
		case basic.CloseReadingConnError:
			logger.Info("直播间连接终止，尝试重联。。")
			err := wcs.Reconnection(resp)
			if err != nil {
				logger.Warn("直播间连接终止，重联失败，退出中。。", slog.Any("err", err))
			}
			canContinue = true
		case basic.CloseAuthFailed:
			logger.Warn("鉴权失败！")
		case basic.CloseActively:
			logger.Warn("直播间连接已关闭")
		case basic.CloseReceivedShutdownMessage:
			logger.Warn("直播间连接被关闭！")
		case basic.CloseTypeUnknown:
			logger.Warn("直播间连接因未知原因关闭！")
		}
		if !canContinue {
			os.Exit(0)
		}
	})
	if err != nil {
		logger.Error("直播间连接失败", slog.Any("err", err))
		return
	}

	tk := time.NewTicker(time.Second * 20)
	go func() {
		for range tk.C {
			err := sdk.AppHeartbeat(resp.GameInfo.GameID)
			if err != nil {
				logger.Warn("Heartbeat fail", slog.Any("err", err), slog.String("roomid", resp.GameInfo.GameID))
			}
		}
	}()
	// 创建http服务
	r := mux.NewRouter()
	r.Methods("Get").Path("/room/{code}/msgs").HandlerFunc(rm.Handler)

	logger.Info("Server starting", slog.Int("port", *flagPort))
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *flagPort))
	if err != nil {
		log.Fatal(err)
	}

	go http.Serve(lis, r)

	// 监听退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			lis.Close()
			rm.Close()
			log.Println("WebsocketClient exit")
			return
		default:
			return
		}
	}
}
