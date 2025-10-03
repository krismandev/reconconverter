package handler

import (
	"time"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func (handler *Handler) BackupCleanerIndodana() {
	logrus.Printf("Job Running... Indodana backup removal")
	conn, client, err := handler.CreateClient(handler.Config.Indodana.SftpSource)
	if err != nil {
		logrus.Printf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
		return
	}

	defer func() {
		client.Close()
		conn.Close()
	}()

	handler.RemoveFiles(conn, client, handler.Config.Indodana.BackupPath)

}

func (handler *Handler) BackupCleanerOvo() {
	logrus.Printf("Job Running... Ovo backup removal")
	conn, client, err := handler.CreateClient(handler.Config.Ovo.SftpSource)
	if err != nil {
		logrus.Printf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
		return
	}

	defer func() {
		client.Close()
		conn.Close()
	}()

	handler.RemoveFiles(conn, client, handler.Config.Ovo.BackupPath)

}

func (handler *Handler) RemoveFiles(sshClient *ssh.Client, sftpClient *sftp.Client, backupPath string) {
	files, err := sftpClient.ReadDir(backupPath)
	if err != nil {
		logrus.Errorf("Failed to read directory: %v", err)
		return
	}

	for _, file := range files {
		originalModTime := file.ModTime()

		now := time.Now()
		diff := now.Sub(originalModTime)
		if diff.Hours() >= 168 {
			logrus.Infof("Delete file %s", file.Name())
			err = sftpClient.Remove(file.Name())
			if err != nil {
				logrus.Errorf("Failed to remove file: %v", err)
				return
			}
		}
	}
}
