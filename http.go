package main

import (
	"PortForwardGo/zlog"
	"bufio"
	"container/list"
	"net"
	"strings"

	proxyprotocol "github.com/pires/go-proxyproto"
)

var http_index map[string]string

func HttpInit() {
	http_index = make(map[string]string)
	zlog.Info("[HTTP] Listening ", Setting.Config.Listen["Http"].Port)
	l, err := net.Listen("tcp", ":"+Setting.Config.Listen["Http"].Port)
	if err != nil {
		zlog.Error("[HTTP] Listen failed , Error: ", err)
		return
	}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go http_handle(c)
	}
}

func LoadHttpRules(i string) {
	Setting.mu.RLock()
	zlog.Info("Loaded [", i, "] (HTTPS)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	http_index[strings.ToLower(Setting.Config.Rules[i].Listen)] = i
	Setting.mu.RUnlock()
}

func DeleteHttpRules(i string) {
	Setting.mu.Lock()
	zlog.Info("Deleted [", i, "] (HTTP)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	delete(http_index, strings.ToLower(Setting.Config.Rules[i].Listen))
	delete(Setting.Config.Rules, i)
	Setting.mu.Unlock()
}

func http_handle(conn net.Conn) {
	headers := bufio.NewReader(conn)
	hostname := ""
	readLines := list.New()
	for {
		bytes, _, error := headers.ReadLine()
		if error != nil {
			conn.Close()
			return
		}
		line := string(bytes)
		readLines.PushBack(line)

		if line == "" {
			break
		}

		if strings.HasPrefix(line, "X-Forward-For: ") == false {
			readLines.PushBack("X-Forward-For: " + ParseAddrToIP(conn.RemoteAddr().String()))
		}

		if strings.HasPrefix(line, "Host: ") {
			hostname = ParseHostToName(strings.TrimPrefix(line, "Host: "))
		}
	}

	if hostname == "" {
		conn.Write([]byte(HttpStatus(503)))
		conn.Write([]byte("\n"))
		conn.Write([]byte(Page503))
		conn.Close()
		return
	}

	i, ok := http_index[hostname]
	if !ok {
		conn.Write([]byte(HttpStatus(503)))
		conn.Write([]byte("\n"))
		conn.Write([]byte(Page503))
		conn.Close()
		return
	}

	Setting.mu.RLock()
	r := Setting.Config.Rules[i]
	Setting.mu.RUnlock()

	if r.Status != "Active" && r.Status != "Created" {
		conn.Write([]byte(HttpStatus(503)))
		conn.Write([]byte("\n"))
		conn.Write([]byte(Page503))
		conn.Close()
		return
	}

	proxy, error := net.Dial("tcp", r.Forward)
	if error != nil {
		conn.Write([]byte(HttpStatus(522)))
		conn.Write([]byte("\n"))
		conn.Write([]byte(Page522))
		conn.Close()
		return
	}

	if r.ProxyProtocolVersion != 0 {
		header := proxyprotocol.HeaderProxyFromAddrs(byte(r.ProxyProtocolVersion), conn.RemoteAddr(), conn.LocalAddr())
		header.WriteTo(proxy)
	}

	for element := readLines.Front(); element != nil; element = element.Next() {
		line := element.Value.(string)
		proxy.Write([]byte(line))
		proxy.Write([]byte("\n"))
	}

	go copyIO(conn, proxy, r.UserID)
	go copyIO(proxy, conn, r.UserID)
}

func ParseAddrToIP(addr string) string {
	var str string
	arr := strings.Split(addr, ":")
	for i := 0; i < (len(arr) - 1); i++ {
		if i != 0 {
			str = str + ":" + arr[i]
		} else {
			str = str + arr[i]
		}
	}
	return str
}

func ParseHostToName(host string) string {
	if strings.Index(host, ":") == -1 {
		return strings.ToLower(host)
	} else {
		return strings.ToLower(strings.Split(host, ":")[0])
	}
}
