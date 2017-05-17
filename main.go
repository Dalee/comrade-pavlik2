package main

import (
	"comrade-pavlik2/pkg/server"
)

func main() {
	m := server.NewServer()
	m.Run()
}
