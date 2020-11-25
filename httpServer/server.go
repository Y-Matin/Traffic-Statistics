package main

import (
	"github.com/mediocregopher/radix.v2/redis"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func main() {
	http.HandleFunc("/path", dataStatistics)
	http.ListenAndServe("127.0.0.1:8888", nil)
}

func dataStatistics(writer http.ResponseWriter, request *http.Request) {
	client, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 1. 用户数量 2.流量总数 3. 当天统计
	now := time.Now()
	strDay := now.Format("20060102")
	uvCount, err := client.Cmd("zcard", "uv_day_"+strDay).Int()
	if err != nil {
		log.Fatal(err)
	}
	pvCount, err := client.Cmd("zcard", "pv_day_"+strDay).Int()
	if err != nil {
		log.Fatal(err)
	}
	bytes, err := ioutil.ReadFile("./showData.html")
	s := string(bytes)
	s = strings.Replace(s, "${pv}", strconv.Itoa(pvCount), -1)
	s = strings.Replace(s, "${uv}", strconv.Itoa(uvCount), -1)

	writer.Write([]byte(s))
}
