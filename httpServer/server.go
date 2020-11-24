package main

import (
	"github.com/mediocregopher/radix.v2/redis"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/path", dataStatistics)
}

func dataStatistics(writer http.ResponseWriter, request *http.Request) {
	client, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	// 1. 用户数量 2.流量总数 3. 当天统计
	//client.Cmd()
}
