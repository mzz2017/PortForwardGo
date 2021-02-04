package main

import (
	"net"
	"testing"
)

func Test(t *testing.T) {
	Setting.Config.Rules = make(map[string]Rule)
	Setting.Config.Users = make(map[string]User)
	Setting.Listener.UDP = make(map[string]*net.UDPConn)
	Setting.Listener.TCP = make(map[string]*net.TCPListener)
	Setting.Config.Rules["test"] = Rule{
		Status:               "Active",
		UserID:               "tester",
		Protocol:             "udp",
		Listen:               "5500",
		Forward:              "localhost:5201",
		ProxyProtocolVersion: 0,
	}
	Setting.Config.Rules["test2"] = Rule{
		Status:               "Active",
		UserID:               "tester",
		Protocol:             "tcp",
		Listen:               "5500",
		Forward:              "localhost:5201",
		ProxyProtocolVersion: 0,
	}
	go LoadUDPRules("test")
	LoadTCPRules("test2")
}
