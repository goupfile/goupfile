package main

import (
	"bytes"
	b64 "encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/skip2/go-qrcode"
)

func handleUpload(w http.ResponseWriter, r *http.Request) {
	fileGroup := FileGroup{
		ID: generateID(6),
	}

	r.ParseMultipartForm(32 << 20) // 32 MB
	fileHeaders := r.MultipartForm.File["files"]
	for _, header := range fileHeaders {
		file, err := header.Open()
		if err != nil {
			http.Error(w, "Unable to read a file", http.StatusBadRequest)
			return
		}
		defer file.Close()
		f, _ := upload(fileGroup.ID, file, header, w, r)
		fileGroup.Files = append(fileGroup.Files, f)
	}

	http.Redirect(w, r, createURL(fileGroup.ID, "v"), http.StatusSeeOther)
}

func upload(fileGroupID string, file multipart.File, header *multipart.FileHeader, w http.ResponseWriter, r *http.Request) (File, error) {
	var Buf bytes.Buffer

	io.Copy(&Buf, file)

	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		log.Println(err)
		log.Println("Making a new uploads directory")
		if err := os.Mkdir("uploads", os.ModePerm); err != nil {
			message := "Could not create uploads directory"
			log.Println(message, err)
			http.Error(w, message, http.StatusInternalServerError)
		}
	}

	id := generateID(6)
	err := ioutil.WriteFile("uploads/"+id, Buf.Bytes(), os.ModePerm)
	if err != nil {
		message := "Unable to save file"
		log.Println(message, err)
		http.Error(w, message, http.StatusInternalServerError)
		return File{}, errors.New(message)
	}

	Buf.Reset()

	fileData := File{
		ID:         id,
		Group:      fileGroupID,
		Name:       header.Filename,
		Size:       header.Size,
		MediaType:  header.Header.Get("Content-Type"),
		UploadDate: time.Now(),
		URL:        createURL(id, "d"),
	}
	log.Println("Uploaded file:", fileData)

	return saveFile(fileData), nil
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	file := getFile(r.URL.Path[len("/d/"):])
	w.Header().Set("Content-Type", file.MediaType+"; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+file.Name)
	http.ServeFile(w, r, "uploads/"+file.ID)
}

func handleView(w http.ResponseWriter, r *http.Request) {
	fileGroup := getFileGroup(r.URL.Path[len("/v/"):])
	fileGroup.QR = b64.StdEncoding.EncodeToString(createQR(createURL(fileGroup.ID, "v")))
	t, err := template.ParseFiles("templates/view.html")
	if err != nil {
		log.Println("Unable to parse template file: ", err)
	}
	t.Execute(w, fileGroup)
}

func createURL(id, action string) string {
	return scheme + "://" + host + port + "/" + action + "/" + id
}

func createQR(url string) []byte {
	var png []byte
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		log.Println("Unable to create QR code: ", err)
	}
	qr.DisableBorder = true
	png, err = qr.PNG(128)
	if err != nil {
		log.Println("Unable to PNG for QR code: ", err)
	}
	return png
}
