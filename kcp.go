package main

import (
	"PortForwardGo/zlog"
	"net"
	kcp "github.com/xtaci/kcp-go"
	"time"
	proxyprotocol "github.com/pires/go-proxyproto"
)

func LoadKCPRules(i string) {
	Setting.mu.Lock()
	ln, err := kcp.ListenWithOptions(":" + Setting.Config.Rules[i].Listen, nil,10,3)
	if err == nil {
		zlog.Info("Loaded [",i,"] (KCP)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	}else{
		zlog.Error("Load failed [",i,"] (KCP) Error: ",err)
		Setting.mu.Unlock()
		SendListenError(i)
		return
	}
	Setting.Listener.KCP[i] = ln
	Setting.mu.Unlock()
	for {
		conn, err := ln.Accept()

		if err != nil {
            if err, ok := err.(net.Error); ok && err.Temporary() {
                continue
            }
			break
		}
		
		Setting.mu.RLock()
		rule := Setting.Config.Rules[i]

		if rule.Status != "Active" && rule.Status != "Created" {
			Setting.mu.RUnlock()
			conn.Close()
			continue
		}

		if Setting.Config.Users[rule.UserID].Used > Setting.Config.Users[rule.UserID].Quota { 			
			Setting.mu.RUnlock()
			conn.Close()
			continue
		}

		Setting.mu.RUnlock()

		go kcp_handleRequest(conn, i, rule)
	}
}

func DeleteKCPRules(i string){
	if _,ok :=Setting.Listener.KCP[i];ok {
		err :=Setting.Listener.KCP[i].Close()
		for err!=nil {
		time.Sleep(time.Second)
		err = Setting.Listener.KCP[i].Close()
		}
	}
	Setting.mu.Lock()
	zlog.Info("Deleted [",i,"] (KCP)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	delete(Setting.Config.Rules,i)
	delete(Setting.Listener.KCP,i)
	Setting.mu.Unlock()
}

func kcp_handleRequest(conn net.Conn, index string, r Rule) {
	proxy, err := kcp.Dial(r.Forward)
	if err != nil {
		_ = conn.Close()
		return
	}

	if r.ProxyProtocolVersion != 0 {
		header := proxyprotocol.HeaderProxyFromAddrs(byte(r.ProxyProtocolVersion),conn.RemoteAddr(),conn.LocalAddr())
		header.WriteTo(proxy)
	}

	go copyIO(conn, proxy, index)
	go copyIO(proxy, conn, index)
}
