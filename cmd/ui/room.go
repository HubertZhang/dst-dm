package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"golang.org/x/exp/slog"

	"hubertzhang.com/dst-dm/room"
)

func startServer(ctx context.Context, c Config, callback func()) error {
	appId, err := strconv.ParseInt(c.AppIdStr, 10, 64)
	if err != nil {
		return fmt.Errorf("appId解析失败: %w", err)
	}
	sdkConfig := live.NewConfig(c.AccessKey, c.AccessToken, appId)

	// 创建sdk实例
	sdk := live.NewClient(sdkConfig)
	basic.DefaultLoggerGenerator = func() *slog.Logger {
		return logger.With("module", "bianka")
	}
	resp, err := sdk.AppStart(c.RoomCode)
	if err != nil {
		logger.Error("服务启动失败", slog.Any("err", err))
		return fmt.Errorf("服务启动失败: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	rm, err := room.New(c.RoomCode, resp, func(wcs *basic.WsClient, _ basic.StartResp, closeType int) {
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
	go func() {
		tk := time.NewTicker(10 * time.Second)
		defer tk.Stop()
		defer cancel()
		for {
			select {
			case <-tk.C:
				err := sdk.AppHeartbeat(resp.GameInfo.GameID)
				if err != nil {
					cancel()
					return
				}
			case <-subCtx.Done():
				return
			}
		}
	}()

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
		sdk.AppEnd(resp.GameInfo.GameID)
		log.Println("WebsocketClient exit")
		callback()
	}()

	return nil
}
