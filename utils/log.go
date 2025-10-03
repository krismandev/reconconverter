package utils

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// CustomJSONFormatter untuk mengontrol urutan field JSON
type CustomJSONFormatter struct{}

type LogFormat struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
}

// Format mengimplementasikan logrus.Formatter
func (f *CustomJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Urutan field yang kita inginkan
	// logdata := struct {
	// 	TimeStamp string
	// }
	// logData := map[string]interface{}{
	// 	"timestamp": entry.Time.Format(time.RFC3339),
	// 	"level":     entry.Level.String(),
	// 	"message":   entry.Message,
	// }
	logData := &LogFormat{
		Time:    entry.Time.Format(time.RFC3339),
		Level:   entry.Level.String(),
		Message: entry.Message,
	}

	// Tambahkan caller jika diaktifkan
	if entry.HasCaller() {
		_, file, line, ok := runtime.Caller(8) // Dapatkan file & line dari caller
		if ok && entry.Level.String() == "error" {
			logData.File = fmt.Sprintf("%s:%d", filepath.Base(file), line) // Simpan hanya file & line
		}
	}

	// Tambahkan fields tambahan jika ada
	// for key, value := range entry.Data {
	// 	logData[key] = value
	// }

	// Encode ke JSON
	logBytes, err := json.Marshal(logData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log entry: %w", err)
	}

	return append(logBytes, '\n'), nil
}
