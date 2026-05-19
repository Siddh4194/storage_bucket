package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	serverAddr := defaultEnv("SERVER_ADDR", "http://localhost:8080")
	bucket := defaultEnv("BUCKET", "loadtest-bucket")
	filePath := defaultEnv("FILE", "SiddhantKadam.pdf")
	totalObjects := 10000
	concurrency := 100

	payload, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("server:    %s\n", serverAddr)
	fmt.Printf("bucket:    %s\n", bucket)
	fmt.Printf("file:      %s (%d bytes)\n", filePath, len(payload))
	fmt.Printf("objects:   %d\n", totalObjects)
	fmt.Printf("workers:   %d\n", concurrency)
	fmt.Println()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        concurrency,
			MaxConnsPerHost:     concurrency,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
		},
		Timeout: 30 * time.Second,
	}

	createBucket(client, serverAddr, bucket)



	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	start := time.Now()
	var mu sync.Mutex
	var okCount, errCount int

	for i := range totalObjects {
		wg.Add(1)
		sem <- struct{}{}

		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()

			key := fmt.Sprintf("obj-%06d", id)
			url := fmt.Sprintf("%s/buckets/%s/objects/%s", serverAddr, bucket, key)

			req, err := http.NewRequest("PUT", url, bytes.NewReader(payload))
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}
			req.Header.Set("Content-Type", "application/pdf")

			resp, err := client.Do(req)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusCreated {
				okCount++
			} else {
				errCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Println()
	fmt.Printf("elapsed:   %v\n", elapsed)
	fmt.Printf("success:   %d\n", okCount)
	fmt.Printf("errors:    %d\n", errCount)
	fmt.Printf("throughput: %.0f objects/sec\n", float64(okCount)/elapsed.Seconds())
}

func createBucket(client *http.Client, addr, name string) {
	body := fmt.Sprintf(`{"name":"%s"}`, name)
	req, err := http.NewRequest("POST", addr+"/buckets", bytes.NewReader([]byte(body)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create bucket request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create bucket: %v\n", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		fmt.Println("bucket ready")
	} else {
		fmt.Fprintf(os.Stderr, "create bucket returned %d\n", resp.StatusCode)
	}
}

func defaultEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
