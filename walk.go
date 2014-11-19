package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func walk(scanPaths []string, fileTypes []string, hashChan chan<- string) {
	var waitGroup sync.WaitGroup

	for _, filePath := range scanPaths {
		waitGroup.Add(1)

		go func(filePath string) {
			defer waitGroup.Done()
			filepath.Walk(filePath, func(filePath string, info os.FileInfo, err error) error {
				if info == nil {
					log.Printf("Error getting file info for %s - %s", filePath, err)
					return nil
				} else if info.IsDir() {
					return nil
				}
				okType := false
				if fileTypes == nil {
					okType = true
				} else {
					lowerFile := strings.ToLower(filePath)
					for _, fileType := range fileTypes {
						if strings.HasSuffix(lowerFile, fileType) {
							log.Printf("Ok file %s is %s", filePath, fileType)
							okType = true
						}
					}
				}

				if okType {
					absPath, err := filepath.Abs(filePath)
					if err != nil {
						log.Printf("Error getting absolute path for %s - %s\n", filePath, err)
						return nil
					}
					hashChan <- absPath
				} else {
					log.Printf("Skipping unwanted file type: %s", filePath)
				}
				return nil
			})
		}(filePath)
	}

	waitGroup.Wait()
	close(hashChan)
}
