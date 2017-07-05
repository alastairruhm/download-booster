package proxy

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	pb "gopkg.in/cheggaaa/pb.v1"

	"strings"

	"log"

	"io"
	"os"
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

// GetFileName parse file name from url given
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
	if res.StatusCode == 301 || res.StatusCode == 302 {
		finalURL := res.Request.URL.String()
		log.Printf("The URL you ended up at is: %v\n", finalURL)
	}
	get.FileName = GetFileName(url)

	get.Header = res.Header
	get.ContentLength = int(res.ContentLength)
	get.MediaType, get.MediaParams, _ = mime.ParseMediaType(res.Header.Get("Content-Disposition"))

	if get.MediaParams["filename"] != "" {
		get.FileName = get.MediaParams["filename"]
	}
	get.HeadRequestDone = true
	if get.Header.Get("Accept-Ranges") != "" {
		log.Printf("Server %s support Range by %s.\n", get.Header.Get("Server"), get.Header.Get("Accept-Ranges"))
		return true, nil
	}
	log.Printf("Server %s doesn't support Range.\n", get.Header.Get("Server"))
	return false, nil
}

func (get *Get) DownloadInParallel(resourceUrl string) error {
	proxyList := [...]string{
		"http://10.34.50.246:1080",
		"http://10.34.50.247:1080",
		"http://10.34.50.248:1080",
		"http://10.34.50.187:1080",
	}

	if !get.HeadRequestDone {
		_, err := get.CheckAcceptRangeSupport(resourceUrl)
		if err != nil {
			return err
		}
	}

	length, _ := strconv.Atoi(get.Header["Content-Length"][0]) // Get the content length from the header request
	limit := len(proxyList)                                    // 10 Go-routines for the process so each downloads 18.7MB
	lenSub := length / limit                                   // Bytes for each Go-routine
	diff := length % limit                                     // Get the remaining for the last request
	pbs := make([]*pb.ProgressBar, limit)
	filePaths := make([]string, limit)
	totals := make([]int64, limit)

	for i := 0; i < limit; i++ {
		get.WG.Add(1)

		min := lenSub * i       // Min range
		max := lenSub * (i + 1) // Max range

		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}

		proxyURL := proxyList[i]
		rangeHeader := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1) // Add the data for the Range header of the form "bytes=0-100"
		tempFileSegName := get.FileName + "." + strconv.Itoa(i)
		// initialize progress bar
		pbs[i] = pb.New(max - min).Prefix(tempFileSegName).SetWidth(160).SetUnits(pb.U_BYTES)
		filePaths[i] = tempFileSegName
		totals[i] = (int64)(max - min)
		go get.downloadSlice(proxyURL, rangeHeader, tempFileSegName)
	}

	pool, err := pb.StartPool(pbs...)
	if err != nil {
		log.Fatal(err)
		return err
	}
	// update bars
	wg := new(sync.WaitGroup)
	for index, bar := range pbs {
		wg.Add(1)

		go func(cb *pb.ProgressBar, index int) {
			showProgress(cb, filePaths[index], totals[index])
			cb.Finish()
			wg.Done()
		}(bar, index)

	}

	// wait download goroutine task over
	get.WG.Wait()
	wg.Wait()
	// close pool
	pool.Stop()

	// assemble slice data
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

		if err != nil {
			log.Fatal(err)
			return err
		}

		_, err = targetFile.Write(data)
		if err != nil {
			log.Fatal(err)
			return err
		}
		err = os.Remove(get.FileName + "." + strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Println("download task over")

	return nil
}

func showProgress(bar *pb.ProgressBar, filePath string, total int64) {

	for {
		file, err := os.OpenFile(
			filePath,
			os.O_CREATE|os.O_RDONLY,
			0644,
		)
		if err != nil {

			fmt.Printf("%+v\n", err)
			log.Fatal(err)
		}
		defer file.Close()

		fi, err := file.Stat()
		if err != nil {
			log.Fatal(err)
		}

		size := fi.Size()
		bar.Set64(size)
		if size == total {
			break
		}

		time.Sleep(time.Millisecond * 200)
	}
}

func (get *Get) downloadSlice(proxyURL string, rangeHeader string, tempFileSegName string) {
	defer get.WG.Done()
	httpProxyURL, err := url.Parse(proxyURL)
	if err != nil {
		log.Fatal("Error parsing Tor proxy URL:", httpProxyURL, ".", err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(httpProxyURL)}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", get.Url, nil)
	req.Header.Add("Range", rangeHeader)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	tempFile, err := os.OpenFile(
		tempFileSegName,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	defer tempFile.Close()
	_, err = io.Copy(tempFile, resp.Body)
	// log.Printf("downlaod %s over", tempFileSegName)
}
