package main

import (
	"sync"
)

type Server struct {
	usedSDRs       map[string]string
	registeredSDRs map[string]string
	rwmu           sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		usedSDRs:       make(map[string]string),
		registeredSDRs: make(map[string]string),
	}
}
