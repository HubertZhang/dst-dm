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
)

var logger *slog.Logger

var (
	flagAccessKey   = flag.String("key", "", "从直播开放平台获取的 accessKey")
	flagAccessToken = flag.String("secret", "", "从直播开放平台获取的 secret")
	flagAppID       = flag.Int64("app-id", 0, "插件 app-id")
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

type Server struct {
	sdk   *live.Client
	Rooms map[string]*Room

	mu *sync.RWMutex
	tk *time.Ticker
}

func (s *Server) Handle(w http.ResponseWriter, req *http.Request) {
	code := mux.Vars(req)["code"]

	if code == "" {
		http.Error(w, "code is empty", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	r, ok := s.Rooms[code]
	s.mu.RUnlock()
	if !ok {
		if err := s.AddRoom(code); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r = s.Rooms[code]
		logger.Info("Added room", slog.String("uname", r.StartResponse.AnchorInfo.Uname), slog.Int("uid", r.StartResponse.AnchorInfo.Uid))
	}
	r.Handler(w, req)
}

func (s *Server) Heartbeat() {
	// 启用项目心跳 20s一次
	// see https://open-live.bilibili.com/document/eba8e2e1-847d-e908-2e5c-7a1ec7d9266f
	s.tk = time.NewTicker(time.Second * 20)
	go func() {
		for range s.tk.C {
			// 如果需要批量心跳，可以使用以下方法
			s.mu.RLock()
			gameIDs := []string{}
			rmIDs := []string{}
			for k, v := range s.Rooms {
				if time.Since(v.LastContactTime) > time.Second*600 {
					rmIDs = append(rmIDs, k)
					continue
				}
				gameIDs = append(gameIDs, v.StartResponse.GameInfo.GameID)
			}
			s.mu.RUnlock()
			if len(gameIDs) != 0 {
				if failed, err := s.sdk.AppBatchHeartbeat(gameIDs); err != nil {
					logger.Warn("Heartbeat fail", slog.Any("err", err), slog.Any("roomid", failed))
				}
			}

			for _, v := range rmIDs {
				r := s.Rooms[v]
				if r != nil {
					logger.Info("Closing room due to timeout", slog.String("uname", r.StartResponse.AnchorInfo.Uname), slog.Int("uid", r.StartResponse.AnchorInfo.Uid))
					r.mu.Lock()
					r.WSClient.Close()
					r.mu.Unlock()
				}
			}

		}
	}()
}

func (s *Server) AddRoom(code string) error {
	startResp, err := s.sdk.AppStart(code)
	if err != nil {
		return err
	}
	r := &Room{
		Code:            code,
		StartResponse:   startResp,
		LastContactTime: time.Now(),
		Messages:        make(chan *proto.Cmd, 100),
		mu:              &sync.RWMutex{},
	}
	dispatcherHandleMap := basic.DispatcherHandleMap{
		proto.OperationMessage: r.Handle,
	}

	onCloseCallback := func(wcs *basic.WsClient, _ basic.StartResp, closeType int) {
		s.mu.Lock()
		delete(s.Rooms, code)
		s.mu.Unlock()
	}

	wsClient, err := basic.StartWebsocket(startResp, dispatcherHandleMap, onCloseCallback, basic.DefaultLoggerGenerator())
	if err != nil {
		return err
	}
	r.WSClient = wsClient
	s.mu.Lock()
	s.Rooms[code] = r
	s.mu.Unlock()

	return nil

}

func (s *Server) Close() error {
	s.tk.Stop()
	rooms := []*Room{}
	for _, v := range s.Rooms {
		rooms = append(rooms, v)
	}
	for _, v := range rooms {
		v.mu.Lock()
		v.WSClient.Close()
		v.mu.Unlock()
	}
	return nil
}

func NewServer(sdk *live.Client) *Server {
	s := &Server{
		sdk:   sdk,
		Rooms: make(map[string]*Room),
		mu:    &sync.RWMutex{},
	}
	go s.Heartbeat()
	return s
}

func main() {
	flag.Parse()

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *flagAccessKey == "" || *flagAccessToken == "" || *flagAppID == 0 {
		fmt.Println("AccessKey 或 Secret 或 AppID 缺失")
		fmt.Println()
		fmt.Println("使用方式：")
		fmt.Println("  start -key=xxx -secret=xxx -app-id=xxx -port=9876")
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

	// 创建http服务
	server := NewServer(sdk)
	r := mux.NewRouter()
	r.Methods("Get").Path("/room/{code}/msgs").HandlerFunc(server.Handle)

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
			server.Close()
			log.Println("WebsocketClient exit")
			return
		default:
			return
		}
	}
}
