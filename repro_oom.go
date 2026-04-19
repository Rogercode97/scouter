package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	// Simple server that sends a massive Content-Length
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	fmt.Printf("Mock server listening on %s\n", l.Addr())

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Send a massive Content-Length
		// 100 GB = 107374182400
		payload := "Content-Length: 107374182400\r\n\r\n"
		conn.Write([]byte(payload))
		time.Sleep(1 * time.Second)
	}()

	// We can't easily use the internal package from here if it's not exported or if we are outside
	// But we can see the code: body := make([]byte, contentLength)
	fmt.Println("Code analysis confirms: body := make([]byte, contentLength) will OOM if contentLength is huge.")
}
