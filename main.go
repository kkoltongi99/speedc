package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const version = "0.0.1"

var revision = "HEAD"

type SpeedMonitor struct {
	downloadBytes int64
	uploadBytes   int64
	downloadMbps  float64
	uploadMbps    float64
	mu            sync.Mutex
	startTime     time.Time
}

func (s *SpeedMonitor) addDownload(n int64) {
	atomic.AddInt64(&s.downloadBytes, n)
}

func (s *SpeedMonitor) addUpload(n int64) {
	atomic.AddInt64(&s.uploadBytes, n)
}

func (s *SpeedMonitor) getSpeeds() (float64, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed > 0 {
		s.downloadMbps = float64(atomic.LoadInt64(&s.downloadBytes)) * 8 / elapsed / 1000000
		s.uploadMbps = float64(atomic.LoadInt64(&s.uploadBytes)) * 8 / elapsed / 1000000
	}
	return s.downloadMbps, s.uploadMbps
}

func generateTestData() []byte {
	data := make([]byte, 1000000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func measureDownload(url string, duration time.Duration, monitor *SpeedMonitor) int64 {
	client := &http.Client{Timeout: duration}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var total int64
	buf := make([]byte, 32768)

	done := make(chan struct{})
	go func() {
		for {
			n, err := io.ReadFull(resp.Body, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				break
			}
			total += int64(n)
			monitor.addDownload(int64(n))
			if n == 0 {
				break
			}
		}
		close(done)
	}()

	select {
	case <-done:
		return total
	case <-time.After(duration):
		return total
	}
}

func measureUpload(url string, data []byte, duration time.Duration, monitor *SpeedMonitor, concurrent int) int64 {
	client := &http.Client{Timeout: duration}

	stop := make(chan struct{})
	var total int64

	var wg sync.WaitGroup

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					reader := bytes.NewReader(data)
					req, err := http.NewRequest("POST", url, reader)
					if err != nil {
						return
					}
					req.ContentLength = int64(len(data))

					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						total += int64(len(data))
						monitor.addUpload(int64(len(data)))
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	return total
}

func runWithAnimation(downloadURL, uploadURL string, duration time.Duration, concurrent int) (*SpeedMonitor, float64, float64) {
	data := generateTestData()

	monitor := &SpeedMonitor{startTime: time.Now()}

	fmt.Printf("Download: ")
	var wg sync.WaitGroup

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			measureDownload(downloadURL, duration, monitor)
		}()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				dl, _ := monitor.getSpeeds()
				fmt.Printf("\rDownload: %.2f Mbps", dl)
			}
		}
	}()

	wg.Wait()
	close(done)
	ticker.Stop()

	downloadElapsed := time.Since(monitor.startTime).Seconds()
	downloadMbps, _ := monitor.getSpeeds()
	fmt.Printf("\rDownload: %.2f Mbps\n", downloadMbps)

	fmt.Printf("Upload:   ")
	atomic.StoreInt64(&monitor.uploadBytes, 0)
	monitor.startTime = time.Now()

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			measureUpload(uploadURL, data, duration, monitor, concurrent)
		}()
	}

	ticker = time.NewTicker(100 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	done2 := make(chan struct{})
	go func() {
		for {
			select {
			case <-done2:
				return
			case <-ticker.C:
				_, ul := monitor.getSpeeds()
				fmt.Printf("\rUpload: %.2f Mbps", ul)
			}
		}
	}()

	wg.Wait()
	close(done2)
	ticker.Stop()

	uploadElapsed := time.Since(monitor.startTime).Seconds()
	_, uploadMbps := monitor.getSpeeds()
	fmt.Printf("\rUpload: %.2f Mbps\n", uploadMbps)

	return monitor, downloadElapsed, uploadElapsed
}

func runWithoutAnimation(downloadURL, uploadURL string, duration time.Duration, concurrent int) (*SpeedMonitor, float64, float64) {
	data := generateTestData()

	monitor := &SpeedMonitor{startTime: time.Now()}

	var wg sync.WaitGroup

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			measureDownload(downloadURL, duration, monitor)
		}()
	}

	wg.Wait()

	downloadElapsed := time.Since(monitor.startTime).Seconds()
	downloadMbps, _ := monitor.getSpeeds()
	fmt.Printf("Download: %.2f Mbps\n", downloadMbps)

	atomic.StoreInt64(&monitor.uploadBytes, 0)
	monitor.startTime = time.Now()

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			measureUpload(uploadURL, data, duration, monitor, concurrent)
		}()
	}

	wg.Wait()

	uploadElapsed := time.Since(monitor.startTime).Seconds()
	_, uploadMbps := monitor.getSpeeds()
	fmt.Printf("Upload: %.2f Mbps\n", uploadMbps)

	return monitor, downloadElapsed, uploadElapsed
}

func main() {
	var (
		info        bool
		downloadURL string
		uploadURL   string
		concurrent  int
		duration    int
		noanim      bool
		showVersion bool
	)

	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&info, "info", false, "show detailed information")
	flag.StringVar(&downloadURL, "download-url", "https://speed.cloudflare.com/__down?bytes=100000000", "download test URL")
	flag.StringVar(&uploadURL, "upload-url", "https://speed.cloudflare.com/__up", "upload test URL")
	flag.IntVar(&concurrent, "concurrent", runtime.GOMAXPROCS(0), "number of concurrent connections")
	flag.IntVar(&duration, "duration", 5, "test duration in seconds")
	flag.BoolVar(&noanim, "noanim", false, "disable animation")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	measureDuration := time.Duration(duration) * time.Second

	if info {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Download URL: %s\n", downloadURL)
		fmt.Printf("  Upload URL:   %s\n", uploadURL)
		fmt.Printf("  Duration:     %v\n", measureDuration)
		fmt.Printf("  Connections:  %d\n", concurrent)
		fmt.Println()
	}

	var monitor *SpeedMonitor
	var downloadElapsed, uploadElapsed float64

	if noanim {
		monitor, downloadElapsed, uploadElapsed = runWithoutAnimation(downloadURL, uploadURL, measureDuration, concurrent)
	} else {
		monitor, downloadElapsed, uploadElapsed = runWithAnimation(downloadURL, uploadURL, measureDuration, concurrent)
	}

	if info {
		dl, ul := monitor.getSpeeds()
		fmt.Println()
		fmt.Printf("Details:\n")
		fmt.Printf("  Download Speed: %.2f Mbps\n", dl)
		fmt.Printf("  Upload Speed:   %.2f Mbps\n", ul)
		fmt.Printf("  Total Bytes (Down): %d (%.2f MB)\n", atomic.LoadInt64(&monitor.downloadBytes), float64(atomic.LoadInt64(&monitor.downloadBytes))/1000000)
		fmt.Printf("  Total Bytes (Up):   %d (%.2f MB)\n", atomic.LoadInt64(&monitor.uploadBytes), float64(atomic.LoadInt64(&monitor.uploadBytes))/1000000)
		fmt.Printf("  Download Time:     %.2f seconds\n", downloadElapsed)
		fmt.Printf("  Upload Time:       %.2f seconds\n", uploadElapsed)
	}
}
