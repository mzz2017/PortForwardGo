package main

import (
	"PortForwardGo/zlog"
	"net/http"
	"net"
	"time"
	"golang.org/x/net/websocket"
	"io"
	proxyprotocol "github.com/pires/go-proxyproto"
)

type Addr struct {
	NetworkType string
	NetworkString string
}

func (this *Addr) Network()string{
	return this.NetworkType
}

func (this *Addr) String()string{
	return this.NetworkString
}

func LoadWSRules(i string){
	Setting.mu.Lock()
	tcpaddress, _ := net.ResolveTCPAddr("tcp", ":"+Setting.Config.Rules[i].Listen)
	ln, err := net.ListenTCP("tcp", tcpaddress)
	if err == nil {
		zlog.Info("Loaded [",i,"] (WebSocket)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	}else{
		zlog.Error("Load failed [",i,"] (Websocket) Error: ",err)
		Setting.mu.Unlock()
		SendListenError(i)
		return
	}
	Setting.Listener.WS[i] = ln
	Setting.mu.Unlock()

	http.Handle("/ws/",websocket.Handler(func(ws *websocket.Conn){
		WS_Handle(i,ws)
	}))

	http.HandleFunc("/",func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		io.WriteString(w,Page404)
		return
	})

	http.Serve(ln,nil)
}

func DeleteWSRules(i string){
	if _,ok :=Setting.Listener.WS[i];ok {
		err :=Setting.Listener.WS[i].Close()
		for err!=nil {
		time.Sleep(time.Second)
		err = Setting.Listener.WS[i].Close()
		}
	}
	Setting.mu.Lock()
	zlog.Info("Deleted [",i,"] (WebSocket)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	delete(Setting.Config.Rules,i)
	delete(Setting.Listener.WS,i)
	Setting.mu.Unlock()
}


func WS_Handle(i string , ws *websocket.Conn){
	ws.PayloadType = websocket.BinaryFrame
	Setting.mu.RLock()
	rule := Setting.Config.Rules[i]

	if rule.Status != "Active" && rule.Status != "Created" {
		Setting.mu.RUnlock()
		ws.Close()
		return
	}

	if Setting.Config.Users[rule.UserID].Used > Setting.Config.Users[rule.UserID].Quota { 			
		Setting.mu.RUnlock()
		ws.Close()
		return
	}

    Setting.mu.RUnlock()

   conn,err := net.Dial("tcp" , rule.Forward)
   if err != nil {
	   ws.Close()
	   return
   }

	if rule.ProxyProtocolVersion != 0 {
		header := proxyprotocol.HeaderProxyFromAddrs(byte(rule.ProxyProtocolVersion),&Addr{
			NetworkType: ws.Request().Header.Get("X-Forward-Protocol"),
			NetworkString: ws.Request().Header.Get("X-Forward-Address"),
		},conn.LocalAddr())
		header.WriteTo(conn)
	}

   go copyIO(ws,conn,i)
   copyIO(conn,ws,i)
}