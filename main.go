package main

import (
	"os"

	"github.com/yunxiaozhao/simple_pbft/network"
)

func main() {
	nodeID := os.Args[1]
	server := network.NewServer(nodeID)

	server.Start()
}
