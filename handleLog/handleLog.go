package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"strconv"

	//"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mgutz/str"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

/**
  解析日志文件，将 一行日志拆分为多个有一定意义的片段，再由对应的 函数处理，入库，提供数据支持，后续根据这些数据进行统计，并实现可视化分析。
*/
type cmdData struct {
	filePath   string
	routineNum int
}

type digData struct {
	url   string
	time  string
	agent string
	refer string
}

type userData struct {
	data digData
	uid  string
}

type urlNode struct {
	unType string //  是 详情页还是 列表页
	unTime string //
	unUrl  string
	unRid  string // resourceId  具体哪个页面
}
type storageBlock struct {
	counterType  string
	storageModel string
	uNode        urlNode
}

/**
  建立redis连接
*/
var wg sync.WaitGroup
var redisPoll *pool.Pool

func init() {
	p, err := pool.New("tcp", "localhost:6379", 10)
	redisPoll = p
	if err != nil {
		// handle err
		fmt.Println("create redis pool failed, error:", err)
		panic(err)
	} else {
		fmt.Println("create redis pool success!")

	}

}

func main() {
	wg.Add(1)
	//1、 获取命令行参数，记录日志
	filePath := flag.String("filePath", "/var/log/nginx/dig.log", "parse file's path")
	routineNum := flag.Int("routineNum", 5, "the count of routine in handle log")
	//logDir:=flag.String("logDir","/home/martin/log/","the logFile's dir generated by this exe")
	flag.Parse()

	var cmd = cmdData{*filePath, *routineNum}

	//2、 初始化一些channel，用于数据传递
	var logChan = make(chan string, 3**routineNum)
	var pvChan = make(chan userData, *routineNum)
	var uvChan = make(chan userData, *routineNum)
	var storageChan = make(chan storageBlock, *routineNum)

	// open log file
	file, err := os.OpenFile(cmd.filePath, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("open file failed ,error=", err)
	}
	defer file.Close()
	//3、 日志消费者
	go readLogFile(file, logChan)
	for i := 0; i < *routineNum; i++ {
		go logConsumer(logChan, pvChan, uvChan)
	}
	//4、 创建PV，UV 统计器
	go pvCounter(pvChan, storageChan)
	go uvCounter(uvChan, storageChan)
	//5、 创建存储器
	go storage(storageChan)

	wg.Wait()
}

func storage(storageChan <-chan storageBlock) {
	for storage := range storageChan {
		prefix := storage.counterType + "_"
		setkeys := []string{
			prefix + "day_" + getTime(storage.uNode.unTime, "day"),
			prefix + "hour_" + getTime(storage.uNode.unTime, "hour"),
			prefix + "min_" + getTime(storage.uNode.unTime, "min"),
			prefix + storage.uNode.unType + "_day_" + getTime(storage.uNode.unTime, "day"),
			prefix + storage.uNode.unType + "_hour_" + getTime(storage.uNode.unTime, "hour"),
			prefix + storage.uNode.unType + "_min_" + getTime(storage.uNode.unTime, "min"),
		}
		rowId := storage.uNode.unRid
		for _, key := range setkeys {
			i, err := redisPoll.Cmd(storage.storageModel, key, 1, rowId).Int()
			if i < 0 || err != nil {
				fmt.Printf("exec \"zincrby %s 1 %s \" failed,error:%v \n", key, rowId, err)
			}

		}
	}
}

func logConsumer(logChan <-chan string, pvChan chan<- userData, uvChan chan<- userData) {
	for log := range logChan {
		digdata := parseLog(log)
		//uid: 模拟uid <==> md5(refer+agent)
		hash := md5.New()
		hash.Write([]byte(digdata.refer + digdata.agent))
		uid := hex.EncodeToString(hash.Sum(nil))
		userdata := userData{digdata, uid}
		//fmt.Printf("%#v\n", userdata)
		pvChan <- userdata
		uvChan <- userdata
	}
}

/*
	解析日志，将一行日志解析 digData结构体
	日志实例：
		127.0.0.1 - - [10/Nov/2020:21:27:52 +0800] "GET /dig?agent=Mozilla%2F5.0+%28Windows+NT+10.0%3B+Win64%3B+x64%29+AppleWebKit%2F537.36+%28KHTML%2C+like+Gecko%29+Chrome%2F86.0.4240.183+Safari%2F537.36&refer=http%3A%2F%2Flocalhost%3A88%2Fgxcms%2Fmovie%2F7791.html&time=%EF%BF%BD&url=http%3A%2F%2Flocalhost%3A88%2Fgxcms%2Fmovie%2F2972.html HTTP/1.1" 200 43 "http://localhost:88/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.183 Safari/537.36" "-"

*/
func parseLog(log string) digData {
	paraBegin := "GET"
	paraEnd := "HTTP/"
	log = strings.TrimSpace(log)
	paraIndexStart := str.IndexOf(log, paraBegin, 0) + len(paraBegin) + 1
	paraIndexEnd := str.IndexOf(log, paraEnd, paraIndexStart) - 1
	paraStr := str.Slice(log, paraIndexStart, paraIndexEnd)
	parse, err := url.Parse("http://localhost" + paraStr)
	if err != nil {
		return digData{}
	}
	values := parse.Query()
	return digData{values.Get("url"), values.Get("time"), values.Get("agent"), values.Get("refer")}
}

/**
统计 点击数，不需要去重
*/
func pvCounter(pvChan chan userData, storageChan chan storageBlock) {
	for uv := range pvChan {
		node := parseUrl(uv)
		storageChan <- storageBlock{"pv", "zincrby", node}
	}

}

func parseUrl(uData userData) urlNode {
	compile := regexp.MustCompile("^.*/(\\d*).html$")

	rId := compile.FindStringSubmatch(uData.data.url)
	var unRid string
	if len(rId) >= 1 {
		unRid = rId[1]
	} else {
		unRid = "-1"
	}
	var urlType string
	if strings.Contains(uData.data.url, "movie") {
		urlType = "movie"
	} else if strings.Contains(uData.data.url, "list") {
		urlType = "list"
	} else {
		urlType = "home"
	}
	node := urlNode{urlType, uData.data.time, uData.data.url, unRid}
	return node
}

/**
统计 user 个数，需要去重
*/
func uvCounter(uvChan <-chan userData, storageChan chan<- storageBlock) {
	for uv := range uvChan {
		// 去重
		hyperLogkey := "uv_hpll_" + getTime(uv.data.time, "day")
		// 利用redis的HyperLogLog 去重。如果 值加入成功，则返回1，没有加入，则返回0.
		ret, err := redisPoll.Cmd("pfadd", hyperLogkey, uv.uid).Int()
		if err != nil {
			fmt.Println("set key faild ,error:", err)
		}

		if ret != 1 {
			continue
		}
		u := parseUrl(uv)
		storageChan <- storageBlock{"uv", "zincrby", u}
	}
}

func getTime(t string, timeType string) string {
	var format string
	switch timeType {
	case "day":
		format = "2006/01/02"
		parse, _ := time.Parse(format, t[0:10])

		year := parse.Year()
		month := parse.Month()
		day := parse.Day()
		return strconv.Itoa(year) + strconv.Itoa(int(month)) + strconv.Itoa(day)

	case "hour":
		format = "2006/01/02 15"
		parse, _ := time.Parse(format, t[0:13])
		year := parse.Year()
		month := parse.Month()
		day := parse.Day()
		hour := parse.Hour()
		return strconv.Itoa(year) + strconv.Itoa(int(month)) + strconv.Itoa(day) + " " + strconv.Itoa(hour)
	case "min":
		format = "2006/01/02 15:04"
		parse, _ := time.Parse(format, t[0:16])
		year := parse.Year()
		month := parse.Month()
		day := parse.Day()
		hour := parse.Hour()
		minute := parse.Minute()
		return strconv.Itoa(year) + strconv.Itoa(int(month)) + strconv.Itoa(day) + " " + strconv.Itoa(hour) + ":" + strconv.Itoa(minute)
	}
	return time.Now().String()
}

func readLogFile(file *os.File, logChan chan<- string) {
	reader := bufio.NewReader(file)
	timeSleep := 3
	var rowCounter int
	for {
		readString, err := reader.ReadString('\n')
		if err != nil {
			// 添加容错机制。等待一会儿，在读
			if err == io.EOF {
				fmt.Printf("readLogFile: read log file EOF ,wait %d \n", timeSleep)
				time.Sleep(time.Duration(timeSleep) * time.Second)
			} else {
				log.Fatal("read  log filed,error:", err)
				break
			}
			continue
		}
		rowCounter++
		if rowCounter%1000 == 0 {
			fmt.Printf("readLogFile: read %d rows\n", rowCounter)
		}
		logChan <- readString
	}

}
