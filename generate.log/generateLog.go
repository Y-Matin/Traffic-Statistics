package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

/**
  生成更多的log，供后面的 go程序分析使用
*/

type urlInfo struct {
	url    string
	target string
	start  int
	end    int
}

var wg sync.WaitGroup
var logChan chan string = make(chan string)

var agentList =make([]string,0)

func main() {

	// 从命令行中 获取 两个参数，1.生成的行数；2.日志文件的路径
	timeStart := time.Now().UnixNano()/1e6
	total := flag.Int("total", 100, "how many rows do you want to create")
	filepath := flag.String("filePath", "/var/log/nginx/dig.log", "log file path")
	flag.Parse()

	// 1. build urlList
	urlInfos := initUrlList()
	var urlList []string
	urlList = buildUrl(urlInfos)
	// 2. generate total rows log
	// for more effective ,use goroutine to  write log to file
	wg.Add(1)
	file, _ := os.OpenFile(*filepath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	// create a goroutine to write
	go writeLogToFile(file, logChan)
	// generate log to chan
	for i := 0; i < *total; i++ {
		var currentPage, referer, agent string
		currentPage = urlList[randIndex(0, len(urlList)-1)]
		referer = urlList[randIndex(0, len(urlList)-1)]
		agent = agentList[randIndex(0, len(agentList)-1)]
		logChan <- generateLog(currentPage, referer, agent)
	}
	close(logChan)
	wg.Wait()
	file.Close()
	fmt.Printf("done : generate %d rows success!\n", *total)
	fmt.Printf("exe cost : %d (毫秒)", time.Now().UnixNano()/1e6-timeStart)
}

func randIndex(start int, end int) int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if start > end {
		return end
	}
	return r.Intn(end-start) + start
}

/**
write log to file
*/
func writeLogToFile(file *os.File, logCh <-chan string) {
	for {
		log, error := <-logCh
		//  when chan is closed, error is failed
		if !error {
			wg.Done()
			return
		}
		_, err := file.WriteString(log+"\n")
		if err != nil {
			fmt.Println("write file fail, error:", err)
		}
	}

}

/**
replace logTemplate func
*/
func generateLog(currentPage, referer, agent string) string {
	logTemplate := `127.0.0.1 - - [10/Nov/2020:21:27:52 +0800] "GET /dig?{$param} HTTP/1.1" 200 43 "http://localhost:88/" "{$agent}" "-"`
	url := url.Values{}
	now := time.Now()
	t := now.Format("2006/01/02 15:04:05")
	url.Add("time", t)
	url.Add("url", currentPage)
	url.Add("refer", referer)
	url.Add("agent", agent)
	paraEncode := url.Encode()
	log := strings.Replace(logTemplate, "{$param}", paraEncode, -1)
	log = strings.Replace(log, "{$agent}", agent, -1)
	return log
}

/**
build urlList
*/
func buildUrl(infos []urlInfo) []string {
	var result = make([]string, 0)
	for _, url := range infos {
		if url.target == "" {
			result = append(result, url.url)
		} else {
			for i := url.start; i <= url.end; i++ {
				url := strings.Replace(url.url, url.target, strconv.Itoa(i), -1)
				result = append(result, url)
			}
		}
	}
	return result
}

func initUrlList() []urlInfo {
	agentList= append(append(agentList, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.183 Safari/537.36"),
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:83.0) Gecko/20100101 Firefox/83.0")

	infos := make([]urlInfo, 0)
	info_1 := urlInfo{url: "http://localhost:88/gxcms/",
		target: "",
		start:  0,
		end:    0,
	}
	info_2 := urlInfo{"http://localhost:88/gxcms/list/{$id}.html",
		"{$id}", 1, 21}
	info_3 := urlInfo{"http://localhost:88/gxcms/movie/{$id}.html", "{$id}", 1, 12924}

	return append(append(append(infos, info_1), info_2), info_3)

}
