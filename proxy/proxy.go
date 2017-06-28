package proxy

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"sync"

	"strings"

	"log"

	"os"

	"github.com/parnurzeal/gorequest"
)

// Get is getter with metadata
type Get struct {
	Url             string
	Header          http.Header
	MediaType       string
	MediaParams     map[string]string
	ContentLength   int
	GetClient       http.Client
	HeadRequestDone bool
	FileName        string
	WG              sync.WaitGroup
}

func GetFileName(url string) string {
	routeSlice := strings.Split(url, "/")
	fileName := routeSlice[len(routeSlice)-1]
	if strings.Index(fileName, "?") != -1 {
		fileName = fileName[0:strings.Index(fileName, "?")]
	}
	return fileName
}

func (get *Get) CheckAcceptRangeSupport(url string) (bool, error) {
	get.Url = url
	res, err := http.Head(get.Url)
	if err != nil {
		return false, err
	}
	fmt.Printf("%+v", res)
	if res.StatusCode == 301 || res.StatusCode == 302 {
		finalURL := res.Request.URL.String()
		fmt.Printf("The URL you ended up at is: %v\n", finalURL)
	}
	get.FileName = GetFileName(url)

	get.Header = res.Header
	get.ContentLength = int(res.ContentLength)
	get.MediaType, get.MediaParams, _ = mime.ParseMediaType(res.Header.Get("Content-Disposition"))
	fmt.Printf("Get %s MediaType:%s, Filename:%s, Length %d.\n", get.Url, get.MediaType, get.MediaParams["filename"], get.ContentLength)
	if get.MediaParams["filename"] != "" {
		get.FileName = get.MediaParams["filename"]
	}
	get.HeadRequestDone = true
	if get.Header.Get("Accept-Ranges") != "" {
		fmt.Printf("Server %s support Range by %s.\n", get.Header.Get("Server"), get.Header.Get("Accept-Ranges"))
		return true, nil
	}
	fmt.Printf("Server %s doesn't support Range.\n", get.Header.Get("Server"))
	return false, nil
}

func (get *Get) DownloadInParallel(url string) error {
	proxyList := [...]string{
		"http://10.34.50.246:1080",
		"http://10.34.50.247:1080",
		"http://10.34.50.248:1080",
		"http://10.34.50.187:1080",
	}

	if !get.HeadRequestDone {
		_, err := get.CheckAcceptRangeSupport(url)
		if err != nil {
			return err
		}
	}

	length, _ := strconv.Atoi(get.Header["Content-Length"][0]) // Get the content length from the header request
	limit := len(proxyList)                                    // 10 Go-routines for the process so each downloads 18.7MB
	lenSub := length / limit                                   // Bytes for each Go-routine
	diff := length % limit                                     // Get the remaining for the last request
	// tempFiles := make([]string, limit)
	// var tempFiles []string
	for i := 0; i < limit; i++ {
		get.WG.Add(1)

		min := lenSub * i       // Min range
		max := lenSub * (i + 1) // Max range

		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}

		go func(min int, max int, i int) {
			// client := &http.Client{}
			// 在此处分配代理池
			// req, _ := http.NewRequest("GET", get.Url, nil)
			req := gorequest.New().Proxy(proxyList[i])
			rangeHeader := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1) // Add the data for the Range header of the form "bytes=0-100"
			// req.Header.Add("Range", rangeHeader)
			// resp, _ := client.Do(req)
			_, bodyBytes, _ := req.Get(get.Url).
				Set("Range", rangeHeader).
				EndBytes()
			// defer resp.Body.Close()

			// reader, _ := ioutil.ReadAll(resp.Body)
			// body[i] = string(reader)

			tempFileSegName := get.FileName + "." + strconv.Itoa(i)
			// tempFiles = append(tempFiles, tempFileSegName)
			ioutil.WriteFile(tempFileSegName, bodyBytes, 0644) // Write to the file i as a byte array
			get.WG.Done()
		}(min, max, i)
	}
	get.WG.Wait()

	targetFile, err := os.OpenFile(
		get.FileName,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer targetFile.Close()

	// merge segment files into one
	for i := 0; i < limit; i++ {

		data, err := ioutil.ReadFile(get.FileName + "." + strconv.Itoa(i))
		log.Printf(get.FileName + "." + strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
			return err
		}

		n, err := targetFile.Write(data)
		if err != nil {
			log.Fatal(err)
			return err
		}
		log.Printf("Wrote %d bytes.\n", n)
		err = os.Remove(get.FileName + "." + strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}
