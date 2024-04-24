package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"hubertzhang.com/dst-dm/proto"
)

var logger *slog.Logger

var (
	flagAccessKey   = flag.String("key", "", "从直播开放平台获取的 accessKey")
	flagAccessToken = flag.String("secret", "", "从直播开放平台获取的 secret")
	flagAppID       = flag.Int64("app-id", 0, "插件 app-id")
	flagPort        = flag.Int("port", 9877, "监听端口")
)

type room struct {
	token     string
	startResp *live.AppStartResponse

	lastUpdate time.Time
	closeFunc  func()
}

type server struct {
	proto.UnimplementedDMServiceServer

	sdk *live.Client

	Rooms map[string]*room

	mu *sync.RWMutex
	tk *time.Ticker
}

func (s *server) bg() {
	s.tk = time.NewTicker(time.Second * 20)
	for range s.tk.C {
		// 如果需要批量心跳，可以使用以下方法
		s.mu.RLock()
		gameIDs := []string{}
		rmIDs := []string{}
		for k, v := range s.Rooms {
			if time.Since(v.lastUpdate) > time.Second*20 {
				rmIDs = append(rmIDs, k)
				continue
			}
			gameIDs = append(gameIDs, v.startResp.GameInfo.GameID)
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
				logger.Info("Closing room due to timeout", slog.String("uname", r.startResp.AnchorInfo.Uname), slog.Int("uid", r.startResp.AnchorInfo.Uid))
				r.closeFunc()
			}
		}
	}
}

func (s *server) Session(server proto.DMService_SessionServer) error {
	startReq, err := server.Recv()
	if err != nil {
		return status.Errorf(codes.Aborted, "failed to receive start request: %s", err)
	}
	if startReq.GetStart() == nil {
		return status.Error(codes.InvalidArgument, "invalid start request")
	}
	code := startReq.GetStart().RoomToken
	logger.Info("Starting session", slog.String("room", code))
	s.mu.RLock()
	_, ok := s.Rooms[code]
	s.mu.RUnlock()
	if ok {
		return status.Error(codes.AlreadyExists, "room already exists")
	}
	startResponse, err := s.sdk.AppStart(code)
	if err != nil {
		fmt.Println("AppStart failed", err)
		return status.Error(codes.InvalidArgument, "failed to start room, check room code")
	}
	logger.Info("Room started", slog.String("room", code), slog.String("uname", startResponse.AnchorInfo.Uname), slog.Int("uid", startResponse.AnchorInfo.Uid))
	d, err := json.Marshal(startResponse)
	if err != nil {
		return err
	}
	err = server.Send(&proto.SessionResponse{
		Response: &proto.SessionResponse_Start{
			Start: &proto.SessionResponse_StartSessionResponse{
				StartApp: d,
			},
		}})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(server.Context())
	r := &room{
		token:     code,
		startResp: startResponse,

		lastUpdate: time.Now(),
		closeFunc:  cancel,
	}

	s.mu.Lock()
	s.Rooms[code] = r
	s.mu.Unlock()

	defer func() {
		logger.Info("Closing room", slog.String("room", code), slog.String("uname", startResponse.AnchorInfo.Uname), slog.Int("uid", startResponse.AnchorInfo.Uid))
		s.mu.Lock()
		delete(s.Rooms, code)
		s.mu.Unlock()
	}()

	msgChan := make(chan *proto.SessionRequest, 5)

	go func() {
		for {
			req, err := server.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				close(msgChan)
				cancel()
				return
			}
			msgChan <- req
		}
	}()
	for {
		select {
		case req := <-msgChan:
			switch req.GetRequest().(type) {
			case *proto.SessionRequest_Heartbeat:
				err = server.Send(&proto.SessionResponse{})
				if err != nil {
					return err
				}
				r.lastUpdate = time.Now()
			default:
				cancel()
				return status.Error(codes.InvalidArgument, "invalid request")
			}
		case <-ctx.Done():
			err = ctx.Err()
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return nil
				}
				return err
			}
			return nil
		}
	}
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

	server := &server{
		sdk:   sdk,
		Rooms: make(map[string]*room),
		mu:    &sync.RWMutex{},
	}
	go server.bg()

	s := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	proto.RegisterDMServiceServer(s, server)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *flagPort))
	if err != nil {
		panic(err)
	}
	go func() {
		err := s.Serve(lis)
		if err != nil {
			fmt.Println("Serve exits with ", err)
		}
	}()

	fmt.Println("Server started at ", *flagPort)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	s.Stop()
	lis.Close()
	fmt.Println("Server stopped")
}
