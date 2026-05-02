package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

const (
	nameOfFolder     = "file"
	filenameParam    = "filename"
	fileMultipartKey = "file"
)

func CreateFile(w http.ResponseWriter, r *http.Request) {
	clientFile, clientFileHeader, err := r.FormFile(fileMultipartKey)
	if err != nil {
		slog.Warn("error when parse multipart file", "error", err)
		http.Error(w, "can't read file from multipart form, try another one", http.StatusBadRequest)
		return
	}
	defer func() {
		err = clientFile.Close()
		if err != nil {
			slog.Error("multipart file close error", "error", err)
		}
	}()
	path, err := getFilePath(clientFileHeader.Filename)
	if err != nil {
		slog.Warn("get file path error", "error", err)
		http.Error(w, "wrong filename", http.StatusBadRequest)
		return
	}
	fileToCreate, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o660)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			slog.Warn("exist file error", "error", err)
			http.Error(w, "file already exists", http.StatusConflict)
			return
		}
		slog.Error("fatal error when create file", "error", err)
		http.Error(w, "fatal error when create file", http.StatusInternalServerError)
		return
	}
	defer func() {
		err = fileToCreate.Close()
		if err != nil {
			slog.Error("close created file error", "error", err)
		}
	}()
	_, err = io.Copy(fileToCreate, clientFile)
	if err != nil {
		slog.Error("copy data from client file to fileserver file error", "error", err)
		http.Error(w, "failed to save file content", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, err = fmt.Fprintf(w, "%v", clientFileHeader.Filename)
	if err != nil {
		slog.Warn("failed to send info to server", "error", err)
	}
}

func ListFiles(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(nameOfFolder)
	if err != nil {
		slog.Error("read directory error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(files) == 0 {
		_, err = fmt.Fprintln(w, "directory is empty")
		if err != nil {
			slog.Warn("failed to send info to server", "error", err)
		}
		return
	}

	for _, file := range files {
		_, err := fmt.Fprintln(w, file.Name())
		if err != nil {
			slog.Warn("failed to send info to server", "error", err)
		}

	}
}

func PrintFile(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue(filenameParam)

	path, err := getFilePath(rawName)
	if err != nil {
		slog.Warn("get file path error", "error", err)
		http.Error(w, "wrong filename", http.StatusBadRequest)
		return
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0o660)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("file not found error", "error", err)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		slog.Error("open file error", "error", err)
		http.Error(w, "fatal open file error", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("close file error", "error", err)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		slog.Error("get stat of file error", "error", err)
		http.Error(w, "fatal get stat of file error", http.StatusInternalServerError)
		return
	}
	if !info.Mode().IsRegular() {
		slog.Warn("file have unsupported type")
		http.Error(w, "file have unsupported type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	_, err = io.Copy(w, file)
	if err != nil {
		slog.Error("copy file to response error", "error", err)
		http.Error(w, "copy file to response error", http.StatusInternalServerError)
		return
	}
}

func DeleteFile(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue(filenameParam)

	path, err := getFilePath(rawName)
	if err != nil {
		slog.Warn("get file path error", "error", err)
		http.Error(w, "wrong filename", http.StatusBadRequest)
		return
	}
	err = os.Remove(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("open file error", "error", err)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		slog.Error("delete file error", "error", err)
		http.Error(w, "fatal delete file error", http.StatusInternalServerError)
		return
	}
}

func UpdateFile(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue(filenameParam)

	path, err := getFilePath(rawName)
	if err != nil {
		slog.Warn("get file path error", "error", err)
		http.Error(w, "wrong filename", http.StatusBadRequest)
		return
	}
	multipartFile, _, err := r.FormFile(fileMultipartKey)
	if err != nil {
		slog.Warn("error when parse multipart file", "error", err)
		http.Error(w, "can't read file from multipart form, try another one", http.StatusBadRequest)
		return
	}
	defer func() {
		err = multipartFile.Close()
		if err != nil {
			slog.Error("multipart file close error", "error", err)
		}
	}()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o660)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("open file error", "error", err)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		slog.Error("fatal error when open file", "error", err)
		http.Error(w, "fatal error when open file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("close file error", "error", err)
		}
	}()

	_, err = io.Copy(file, multipartFile)
	if err != nil {
		slog.Error("copy raw body to file error", "error", err)
		http.Error(w, "failed to save file content", http.StatusInternalServerError)
		return
	}
}
