package main

import (
	"errors"
	"net"
	"time"
	"PortForwardGo/zlog"
	proxyprotocol "github.com/pires/go-proxyproto"
)

type UDPDistribute struct {
	Established bool
	Conn        *(net.UDPConn)
	RAddr       net.Addr
	LAddr       net.Addr
	Cache       chan []byte
}

func NewUDPDistribute(conn *(net.UDPConn), addr net.Addr) (*UDPDistribute) {
	return &UDPDistribute{
		Established: true,
		Conn:        conn,
		RAddr:       addr,
		LAddr:       conn.LocalAddr(),
		Cache:       make(chan []byte, 16),
	}
}

func (this *UDPDistribute) Close() (error) {
	this.Established = false
	return nil
}

func (this *UDPDistribute) Read(b []byte) (n int, err error) {
	if !this.Established {
		return 0, errors.New("udp distrubute has closed")
	}

	select {
	case <-time.After(16 * time.Second):
		return 0, errors.New("udp distrubute read timeout")
	case data := <-this.Cache:
		n := len(data)
		copy(b, data)
		return n, nil
	}
}

func (this *UDPDistribute) Write(b []byte) (n int, err error) {
	if !this.Established {
		return 0, errors.New("udp distrubute has closed")
	}
	return this.Conn.WriteTo(b, this.RAddr)
}

func (this *UDPDistribute) RemoteAddr() (net.Addr) {
	return this.RAddr
}

func (this *UDPDistribute) LocalAddr() (net.Addr) {
	return this.LAddr
}

func (this *UDPDistribute) SetDeadline(t time.Time) error {
	return this.Conn.SetDeadline(t)
}
 
func (this *UDPDistribute) SetReadDeadline(t time.Time) error {
	return this.Conn.SetReadDeadline(t)
}

func (this *UDPDistribute) SetWriteDeadline(t time.Time) error {
	return this.Conn.SetWriteDeadline(t)
}
 
func LoadUDPRules(i string){

	clientc := make(chan net.Conn)

	Setting.mu.Lock()
	address, _ := net.ResolveUDPAddr("udp", ":"+Setting.Config.Rules[i].Listen)
	ln, err := net.ListenUDP("udp", address)
	if err == nil {
		zlog.Info("Loaded [",i,"] (UDP)", Setting.Config.Rules[i].Listen, " => ", Setting.Config.Rules[i].Forward)
	}else{
		zlog.Error("Load failed [",i,"] (UDP) Error: ",err)
		SendListenError(i)
		Setting.mu.Unlock()
		return
	}

	Setting.Listener.UDP[i] = ln
	Setting.mu.Unlock()

	go AcceptUDP(ln, clientc)

	for {
		var conn net.Conn = nil
		select {
		case conn = <-clientc:
			if conn == nil {
				break
			}
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

		go udp_handleRequest(conn,i,rule)
	}
}
		
func DeleteUDPRules(i string){
	if _,ok :=Setting.Listener.UDP[i];ok{
	err :=Setting.Listener.UDP[i].Close()
	for err!=nil {
	time.Sleep(time.Second)
	err = Setting.Listener.UDP[i].Close()
	}
    }
	Setting.mu.Lock()
	delete(Setting.Config.Rules,i)
	delete(Setting.Listener.UDP,i)
	Setting.mu.Unlock()
}

func udp_handleRequest(conn net.Conn, index string, r Rule) {
    	proxy, err := ConnUDP(r.Forward)
		if err != nil {
			conn.Close()
			return
		}

		if r.ProxyProtocolVersion != 0 {
			header := proxyprotocol.HeaderProxyFromAddrs(byte(r.ProxyProtocolVersion),conn.RemoteAddr(),conn.LocalAddr())
			header.WriteTo(proxy)
		}

		go copyIO(conn,proxy,index)
		go copyIO(proxy,conn,index)
}

func AcceptUDP(serv *net.UDPConn, clientc chan net.Conn) {

	table := make(map[string]*UDPDistribute)

	for {
		serv.SetDeadline(time.Now().Add(16 * time.Second))

		buf := make([]byte, 32 * 1024)
		n, addr, err := serv.ReadFrom(buf)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				continue
			}
			clientc <- nil
			return
		}
		buf = buf[:n]

		if d, ok := table[addr.String()]; ok {
			if d.Established {
				d.Cache <- buf
				continue
			} else {
				delete(table, addr.String())
			}
		}
		conn := NewUDPDistribute(serv, addr)
		table[addr.String()] = conn
		conn.Cache <- buf
		clientc <- conn
	}
}

func ConnUDP(address string) (net.Conn, error) {
	conn, err := net.DialTimeout("udp", address, 10 * time.Second)
	if err != nil {
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(60 * time.Second))
	return conn, nil
}
