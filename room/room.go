package room

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"github.com/vtb-link/bianka/proto"
	"golang.org/x/exp/slog"
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
	r.WSClient.Logger().Debug("Returned msgs to DST", slog.Int("count", len(msgs)))
	err := json.NewEncoder(w).Encode(msgs)
	if err != nil {
		r.WSClient.Logger().Warn("Error encoding messages", slog.Any("err", err))
	}
}

func (r *Room) Handle(wsClient *basic.WsClient, msg *proto.Message) error {
	// sdk提供了自动解析消息的方法，可以快速解析为对应的cmd和data
	// 具体的cmd 可以参考 proto/cmd.go
	cmd, data, err := proto.AutomaticParsingMessageCommand(msg.Payload())
	if err != nil {
		return err
	}
	r.Messages <- &proto.Cmd{
		Cmd:  cmd,
		Data: data,
	}
	return nil
}

func New(code string, startResp *live.AppStartResponse, onCloseCallback func(wcs *basic.WsClient, _ basic.StartResp, closeType int)) (*Room, error) {
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

	wsClient, err := basic.StartWebsocket(startResp, dispatcherHandleMap, onCloseCallback, basic.DefaultLoggerGenerator())
	if err != nil {
		return nil, err
	}
	r.WSClient = wsClient
	return r, nil
}

func (r *Room) Close() {
	r.mu.Lock()
	r.WSClient.Close()
	r.mu.Unlock()
}
