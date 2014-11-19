package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type upload_response struct {
	Status int
	Error  string
}

type ruler_response struct {
	Status   int
	Location string
}

func processUploads(cache_dir, base_endpoint, access_token string, uploadChan chan UploadJob, doneChan chan<- UploadJob) {
	var wg sync.WaitGroup
	for job := range uploadChan {
		wg.Add(1)
		go func(job UploadJob) {
			defer wg.Done()
			log.Printf("Uploading %s\n", job.filePath)
			api_url, err := url.Parse(base_endpoint)

			if err != nil {
				panic(err)
			}

			file, err := os.Open(job.filePath)
			if err != nil {
				log.Printf("Couldn't open %s for uploading: %s.\n", job.filePath, err)
				return
			}

			newUpload := true

			if newUpload {
				api_url, err := url.Parse("https://services.picturelife.com/")

				api_url.Path = "/ruler/" + job.fileHash + "_abc123.JPG"

				req, err := http.NewRequest("HEAD", api_url.String(), nil)
				if err != nil {
					log.Printf("Error forming HEAD ruler request for %s - %s\n", job.filePath, err)
					return
				}

				req.Header.Set("Authorization", "Bearer "+access_token)

				client := http.Client{}

				response, err := client.Do(req)
				if err != nil {
					log.Printf("Error uploading %s - %s\n", job.filePath, err)
					return
				}

				exists := response.Header["X-Existing"]
				if exists[0] == "true" {
					log.Printf("Image already exists: %s", job.filePath)
					return
				}

				log.Printf("New method response to HEAD ruler: %s", response.Header["X-Existing"])

				buf := new(bytes.Buffer)
				if _, err = io.Copy(buf, file); err != nil {
					log.Printf("Error copying data between file and form: %s - %s\n", job.filePath, err)
					return
				}

				req, err = http.NewRequest("PUT", api_url.String(), buf)
				if err != nil {
					log.Printf("Error forming upload request for %s - %s\n", job.filePath, err)
					return
				}
				req.Header.Set("Authorization", "Bearer "+access_token)

				client = http.Client{}

				log.Printf("PUTing file: %s", job.filePath)

				response, err = client.Do(req)
				if err != nil {
					log.Printf("Error uploading %s - %s\n", job.filePath, err)
					return
				}

				resp := ruler_response{}
				body, err := ioutil.ReadAll(response.Body)
				json.Unmarshal(body, &resp)

				if resp.Status != 20000 {
					log.Printf("Unable to PUT file to /ruler/: %s: %s", job.filePath, body)
					return
				}

				log.Printf("Finished PUT /ruler/: %s", resp.Location)

				api_url, err = url.Parse("https://upload.picturelife.com/media/create")

				var formBuffer bytes.Buffer

				formWriter := multipart.NewWriter(&formBuffer)
				formWriter.WriteField("access_token", access_token)
				formWriter.WriteField("url", resp.Location)
				formWriter.WriteField("signature", job.fileHash)
				formWriter.WriteField("filename", job.filePath[strings.LastIndex(job.filePath, "/")+1:])

				formWriter.Close()

				req, err = http.NewRequest("POST", api_url.String(), &formBuffer)
				if err != nil {
					log.Printf("Error forming final media create request for %s - %s\n", job.filePath, err)
					return
				}

				req.Header.Set("Content-Type", formWriter.FormDataContentType())

				client = http.Client{}

				response, err = client.Do(req)
				if err != nil {
					log.Printf("Error uploading %s - %s\n", job.filePath, err)
					return
				}

				uploadresp := upload_response{}
				body, err = ioutil.ReadAll(response.Body)
				json.Unmarshal(body, &uploadresp)

				if uploadresp.Status != 20000 {
					log.Printf("Unable to upload %s - %s\n", job.filePath, uploadresp.Error)
					return
				}

				log.Printf("Uploaded: %s - %s", job.filePath, body)

			} else {
				api_url.Path = "/medias/create"

				var formBuffer bytes.Buffer

				formWriter := multipart.NewWriter(&formBuffer)
				formWriter.WriteField("access_token", access_token)

				fileWriter, err := formWriter.CreateFormFile("file", job.filePath)

				if err != nil {
					log.Printf("Couldn't add a multipart form field for %s: %s\n", job.filePath, err)
					return
				}

				if _, err = io.Copy(fileWriter, file); err != nil {
					log.Printf("Error copying data between file and form: %s - %s\n", job.filePath, err)
					return
				}

				formWriter.Close()

				req, err := http.NewRequest("POST", api_url.String(), &formBuffer)
				if err != nil {
					log.Printf("Error forming upload request for %s - %s\n", job.filePath, err)
					return
				}

				req.Header.Set("Content-Type", formWriter.FormDataContentType())

				client := http.Client{}

				response, err := client.Do(req)
				if err != nil {
					log.Printf("Error uploading %s - %s\n", job.filePath, err)
					return
				}

				uploadresp := upload_response{}
				body, err := ioutil.ReadAll(response.Body)
				json.Unmarshal(body, &uploadresp)

				if uploadresp.Status != 20000 {
					log.Printf("Unable to upload %s - %s\n", job.filePath, uploadresp.Error)
					return
				}

				log.Printf("Uploaded: %s - %s", job.filePath, body)
			}

			job.uploaded = true

			job.AddToCache(cache_dir)

			doneChan <- job
		}(job)
	}
	wg.Wait()
	close(doneChan)
}
