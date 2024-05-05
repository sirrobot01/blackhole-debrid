package runner

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"goBlack/common"
	"goBlack/pkg"
	"goBlack/pkg/debrid"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func fileReady(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err) // Returns true if the file exists
}

func checkFileLoop(wg *sync.WaitGroup, dir string, file pkg.File, ready chan<- pkg.File) {
	defer wg.Done()
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()
	path := filepath.Join(dir, file.Path)
	for {
		select {
		case <-ticker.C:
			if fileReady(path) {
				ready <- file
				return
			}
		}
	}
}

func ProcessFiles(config *common.TorrentConfig, torrent *pkg.Torrent) {
	var wg sync.WaitGroup
	files := torrent.Files
	ready := make(chan pkg.File, len(files))

	log.Println("Checking files...")

	for _, file := range files {
		wg.Add(1)
		go checkFileLoop(&wg, config.Debrid.Folder, file, ready)
	}

	go func() {
		wg.Wait()
		close(ready)
	}()

	for r := range ready {
		log.Println("File is ready:", r.Name)
		CreateSymLink(config, torrent)

	}
	go torrent.Cleanup(config, true)
	fmt.Printf("%s downloaded\nFiles Count: %d", torrent.Name, len(torrent.Files))
}

func CreateSymLink(config *common.TorrentConfig, torrent *pkg.Torrent) {
	path := filepath.Join(config.CompletedFolder, torrent.Folder)
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Printf("Failed to create directory: %s\n", path)
	}
	for _, file := range torrent.Files {
		// Combine the directory and filename to form a full path
		fullPath := filepath.Join(config.CompletedFolder, file.Path)

		// Create a symbolic link if file doesn't exist
		_ = os.Symlink(filepath.Join(config.Debrid.Folder, file.Path), fullPath)
	}
}

func watchFiles(watcher *fsnotify.Watcher, events map[string]time.Time) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				if filepath.Ext(event.Name) == ".torrent" || filepath.Ext(event.Name) == ".magnet" {
					events[event.Name] = time.Now()
				}

			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("ERROR:", err)
		}
	}
}

func processFilesDebounced(config *common.TorrentConfig, db debrid.Service, events map[string]time.Time, debouncePeriod time.Duration) {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()

	for range ticker.C {
		for file, lastEventTime := range events {
			if time.Since(lastEventTime) >= debouncePeriod {
				log.Printf("Torrent file detected: %s", file)
				// Process the torrent file
				torrent, err := db.Process(file)
				if err != nil || torrent == nil {
					if torrent != nil {
						// remove torrent file
						torrent.Cleanup(config, true)
					}
					log.Printf("Error processing torrent file: %s", err)
				}
				if err == nil && torrent != nil && len(torrent.Files) > 0 {
					go ProcessFiles(config, torrent)
				}
				delete(events, file) // remove file from channel

			}
		}
	}
}

func RunArr(conf *common.TorrentConfig, db debrid.Service) {
	log.Printf("Watching: %s", conf.WatchFolder)
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(w *fsnotify.Watcher) {
		err := w.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(w)
	events := make(map[string]time.Time)

	go watchFiles(w, events)
	if err = w.Add(conf.WatchFolder); err != nil {
		log.Println("Error Watching folder:", err)
		return
	}

	processFilesDebounced(conf, db, events, 1*time.Second)
}

func Run(config *common.Config) {
	log.Print("[*] BlackHole running")
	var wg sync.WaitGroup
	debridConf := config.Debrid
	db := debrid.NewDebrid(debridConf.Name, debridConf.Host, debridConf.APIKey, debridConf.DownloadUncached)
	for _, arr := range config.Arrs {
		wg.Add(1)
		defer wg.Done()
		conf := &common.TorrentConfig{
			Debrid:          config.Debrid,
			WatchFolder:     arr.WatchFolder,
			CompletedFolder: arr.CompletedFolder,
		}
		go RunArr(conf, db)
	}
	wg.Wait()

}
