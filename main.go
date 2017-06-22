package main

import (
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type Get struct {
	Url           string
	Header        http.Header
	MediaType     string
	MediaParams   map[string]string
	ContentLength int
	GetClient     http.Client
	WG            sync.WaitGroup
}

// var wg sync.WaitGroup

func main() {
	var get Get
	get.Url = "http://7b1h1l.com1.z0.glb.clouddn.com/bryce.jpg"
	res, _ := http.Head(get.Url) // 187 MB file of random numbers per line

	get.Header = res.Header
	get.ContentLength = int(res.ContentLength)
	get.MediaType, get.MediaParams, _ = mime.ParseMediaType(res.Header.Get("Content-Disposition"))
	log.Printf("Get %s MediaType:%s, Filename:%s, Length %d.\n", get.Url, get.MediaType, get.MediaParams["filename"], get.ContentLength)

	if get.Header.Get("Accept-Ranges") != "" {
		log.Printf("Server %s support Range by %s.\n", get.Header.Get("Server"), get.Header.Get("Accept-Ranges"))
	} else {
		log.Printf("Server %s doesn't support Range.\n", get.Header.Get("Server"))
		os.Exit(-1)
	}

	length, _ := strconv.Atoi(get.Header["Content-Length"][0]) // Get the content length from the header request
	limit := 2                                                 // 10 Go-routines for the process so each downloads 18.7MB
	lenSub := length / limit                                   // Bytes for each Go-routine
	diff := length % limit                                     // Get the remaining for the last request
	body := make([]string, 11)                                 // Make up a temporary array to hold the data to be written to the file
	for i := 0; i < limit; i++ {
		get.WG.Add(1)

		min := lenSub * i       // Min range
		max := lenSub * (i + 1) // Max range

		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}

		go func(min int, max int, i int) {
			client := &http.Client{}
			// 在此处分配代理池
			req, _ := http.NewRequest("GET", get.Url, nil)
			rangeHeader := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1) // Add the data for the Range header of the form "bytes=0-100"
			req.Header.Add("Range", rangeHeader)
			resp, _ := client.Do(req)
			defer resp.Body.Close()

			reader, _ := ioutil.ReadAll(resp.Body)
			body[i] = string(reader)
			ioutil.WriteFile(strconv.Itoa(i), []byte(string(body[i])), 0x777) // Write to the file i as a byte array
			get.WG.Done()
			// ioutil.WriteFile("new_oct.png", []byte(string(body)), 0x777)
		}(min, max, i)
	}
	get.WG.Wait()
}
