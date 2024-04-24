package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "hubertzhang.com/dst-dm/proto"
	"hubertzhang.com/dst-dm/room"
)

func startServer(ctx context.Context, roomCode string, callback func()) error {
	conn, err := grpc.NewClient("127.0.0.1:9877", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("grpc client error", slog.Any("err", err))
		return err
	}
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	subCtx, cancel := context.WithCancel(ctx)
	client := pb.NewDMServiceClient(conn)

	s, err := client.Session(subCtx)
	if err != nil {
		cancel()
		logger.Error("grpc client error", slog.Any("err", err))
		return err
	}

	err = s.Send(&pb.SessionRequest{
		Request: &pb.SessionRequest_Start{
			Start: &pb.SessionRequest_StartSessionRequest{
				RoomToken: roomCode,
			},
		},
	})
	if err != nil {
		logger.Error("grpc client error", slog.Any("err", err))
		cancel()
		return err
	}

	ret, err := s.Recv()
	if err != nil {
		logger.Error("grpc client error", slog.Any("err", err))
		cancel()
		return err
	}
	go func() {
		tk := time.NewTicker(10 * time.Second)
		defer tk.Stop()
		defer cancel()
		for {
			select {
			case <-tk.C:
				err := s.Send(&pb.SessionRequest{
					Request: &pb.SessionRequest_Heartbeat{},
				})
				if err != nil {
					logger.Error("grpc client error", slog.Any("err", err))
					return
				}
				_, err = s.Recv()
				if err != nil {
					logger.Error("grpc client error", slog.Any("err", err))
					return
				}
			case <-subCtx.Done():
				return
			}
		}
	}()
	respData := ret.GetStart().GetStartApp()
	resp := &live.AppStartResponse{}
	err = json.Unmarshal(respData, resp)
	if err != nil {
		logger.Error("服务启动失败", slog.Any("err", err))
		cancel()
		return err
	}
	rm, err := room.New(roomCode, resp, func(wcs *basic.WsClient, _ basic.StartResp, closeType int) {
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
			cancel()
		}
	})
	if err != nil {
		logger.Error("直播间连接失败", slog.Any("err", err))
		cancel()
		return err
	}

	// 创建http服务
	r := mux.NewRouter()
	r.Methods("Get").Path("/room/{code}/msgs").HandlerFunc(rm.Handler)

	logger.Info("Server starting", slog.Int("port", *flagPort))
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *flagPort))
	if err != nil {
		cancel()
		log.Fatal(err)
	}

	go http.Serve(lis, r)
	go func() {
		<-subCtx.Done()
		lis.Close()
		rm.Close()
		log.Println("WebsocketClient exit")
		callback()
	}()

	return nil
}
