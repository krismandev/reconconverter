package handler

import (
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"reconconverter/config"
	"reconconverter/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/ssh"
	"gopkg.in/gomail.v2"
)

type Handler struct {
	Config     *config.Config
	Client     *ssh.Client
	MailSender mail.Sender
	Assets     *mail.Assets
}

var reasonsMap = map[string]string{
	"notExistsError":   "File tidak ditemukan",
	"invalidFileError": "File tidak valid",
	"emptyFileError":   "File kosong",
	"unknownError":     "Unknown Error",
	"directoryError":   "Directory Error",
	"internalError":    "Internal Error",
}

var indodanaFormat []string = []string{"NO", "MERCHANT NAME", "TRANSACTION DATE", "TRANSIDMERCHANT", "CUSTOMER NAME", "AMOUNT", "FEE", "TAX", "MERCHANT SUPPORT", "PAY TO MERCHANT", "PAY OUT DATE", "TRANSACTION TYPE", "TENURE"}

func NewHandler(config *config.Config, assets *mail.Assets) *Handler {

	dialer := gomail.NewDialer(config.Smtp.Host, config.Smtp.Port, config.Smtp.User, config.Smtp.Password)
	// dialer.Auth = smtp.PlainAuth("", config.Smtp.User, config.Smtp.Password, config.Smtp.Host)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	dialer.SSL = false

	if conn, err := dialer.Dial(); err != nil {
		logrus.Fatalf("failed to connect to smtp server: %v", err)
	} else if err := conn.Close(); err != nil {
		logrus.Fatalf("failed to close connection : %v", err)
	}

	logrus.Info("Connected to smtp")

	return &Handler{
		Config:     config,
		Assets:     assets,
		MailSender: dialer,
	}
}

func (handler *Handler) OvoHandler() {
	channelName := "ovo"
	logrus.Printf("Job Running... Ovo")
	conn, client, err := handler.CreateClient(handler.Config.Ovo.SftpSource)
	if err != nil {
		logrus.Fatalf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
		return
	}

	defer client.Close()
	defer conn.Close()

	files, err := client.ReadDir(handler.Config.Ovo.SourcePath)
	if err != nil {
		logrus.Fatalf("Failed to read directory: %v", err)
		return
	}

	connDest, clientDest, err := handler.CreateClient(handler.Config.Ovo.SftpDestination)
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
		clientDest.Close()
		connDest.Close()
	}()

	for _, file := range files {
		var err error
		if file.IsDir() {
			continue // skip subdirectories
		}

		remoteFileSourcePath := handler.Config.Ovo.SourcePath + "/" + file.Name()
		remoteFile, err := client.Open(remoteFileSourcePath)
		if err != nil {
			logrus.Printf("Failed to open file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		defer remoteFile.Close()

		localPathBefore := handler.Config.TempFolder + "/before/" + channelName + "/"

		if err := os.MkdirAll(localPathBefore, 0755); err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}
		localPathBefore = localPathBefore + file.Name()
		localFileBefore, err := os.Create(localPathBefore)
		if err != nil {
			logrus.Printf("Failed to create local file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		defer localFileBefore.Close()

		_, err = io.Copy(localFileBefore, remoteFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		} else {
			logrus.Infof("Downloaded: %v", file.Name())
		}

		f, err := excelize.OpenFile(localFileBefore.Name())
		if err != nil {
			logrus.Fatalf("Got error %v", err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		sheetList := f.GetSheetList()

		sheet := sheetList[0]

		// var header []string
		var content [][]string

		rows, err := f.GetRows(sheet)

		if err != nil {
			logrus.Errorf("Got error when get rows %v", err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		var countBefore int
		for idx, each := range rows {
			if idx > 0 {
				countBefore++
			}
			content = append(content, each)
		}

		outputDir := "./tmp/after/" + channelName

		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}
		arrName := strings.Split(localFileBefore.Name(), "/")

		re := regexp.MustCompile(`(\d{2})-(\d{2})-(\d{4})`)

		newFilename := re.ReplaceAllString(arrName[len(arrName)-1], "$3$2$1")
		newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

		newFilename = newFilename + ".csv"
		localFileAfter := outputDir + "/" + newFilename
		newFile, err := os.Create(localFileAfter)
		if err != nil {
			logrus.Errorf("Error when create file %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		writer := csv.NewWriter(newFile)
		writer.Comma = ';'
		columnNum := 0

		for idx, each := range content {
			if idx == 0 {
				columnNum = len(each)
			}
			// exclude row terakhir (row summary)
			if idx < len(content)-1 {
				// jika kolom terakhir tidak ada datanya
				if len(each) == (columnNum - 1) {
					each = append(each, "")
				}
				if err := writer.Write(each); err != nil {
					logrus.Errorf("Failed to write file %v: %v", newFile.Name(), err)
					handler.OnErrorHandler("internalError", channelName, err)
					continue
				}
			}
		}

		writer.Flush()

		fmt.Println("OVO file " + localFileBefore.Name() + " converted to ---->  " + newFilename + " successfully")

		dstFile, err := clientDest.Create(handler.Config.Ovo.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Errorf("Failed to put file %v to sftp server. Err: %v", newFilename, err.Error())
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}

		defer dstFile.Close()

		if _, err := newFile.Seek(0, 0); err != nil {
			logrus.Errorf("Failed to seek file %v: %v", newFile.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		_, err = io.Copy(dstFile, newFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %v to sftp server. Err: %v", newFile.Name(), err.Error())
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		// read again to count row after converted
		convertedFile, err := clientDest.Open(handler.Config.Ovo.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Fatalf("Failed to read file (converted) %s: %v", newFilename, err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
		}

		defer convertedFile.Close()

		reader := csv.NewReader(convertedFile)
		reader.Comma = ';'
		convertedRecords, err := reader.ReadAll()
		if err != nil {
			logrus.Errorf("Failed to read csv: %v", err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		var countAfter int = len(convertedRecords) - 1

		logrus.Printf("Count before: %d", countBefore)
		logrus.Printf("Count after: %d", countAfter)

		logrus.Printf("Success converting file")

		handler.OnSuccessHandler("", channelName, countBefore, countAfter)

		err = os.Remove(localPathBefore)
		if err != nil {
			logrus.Errorf("Failed to remove local file %v", err)
		}
		err = os.Remove(localFileAfter)
		if err != nil {
			logrus.Errorf("Failed to remove local file %v", err)
		}

		backupPath := handler.Config.Ovo.BackupPath + "/" + file.Name()
		err = client.Rename(remoteFileSourcePath, backupPath)
		if err != nil {
			logrus.Errorf("Failed to remove remote file %v", err)
		}
	}
}

func (handler *Handler) IndodanaHandler() {
	channelName := "indodana"
	logrus.Printf("Job Running... Indodana")
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

	files, err := client.ReadDir(handler.Config.Indodana.SourcePath)
	if err != nil {
		logrus.Fatalf("Failed to read directory: %v", err)
		return
	}

	connDest, clientDest, err := handler.CreateClient(handler.Config.Indodana.SftpDestination)
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
		clientDest.Close()
		connDest.Close()
	}()

	for _, file := range files {
		var err error
		if file.IsDir() {
			continue // skip subdirectories
		}

		remoteFileSourcePath := handler.Config.Indodana.SourcePath + "/" + file.Name()
		remoteFile, err := client.Open(remoteFileSourcePath)
		if err != nil {
			logrus.Printf("Failed to open file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		// info, err := client.Stat(handler.Config.Indodana.SourcePath + "/" + file.Name())
		// if err != nil {
		// 	handler.OnErrorHandler("",channelName, err)
		// 	continue
		// }

		defer remoteFile.Close()

		localPathBefore := handler.Config.TempFolder + "/before/" + channelName + "/"

		if err := os.MkdirAll(localPathBefore, 0755); err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}
		localPathBefore = localPathBefore + file.Name()
		localFileBefore, err := os.Create(localPathBefore)
		if err != nil {
			logrus.Printf("Failed to create local file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		defer localFileBefore.Close()

		_, err = io.Copy(localFileBefore, remoteFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %s: %v", file.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		} else {
			logrus.Infof("Downloaded: %v", file.Name())
		}

		f, err := excelize.OpenFile(localFileBefore.Name())
		if err != nil {
			logrus.Fatalf("Got error %v", err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		// var header []string
		var content [][]string

		rows, err := f.GetRows("Ledger")
		if err != nil {
			logrus.Errorf("Got error when get rows %v", err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		var countBefore int
		var errorOnContent bool = false
	firstLoop:
		for idx, each := range rows {
			if idx == 0 {
				if len(each) != len(indodanaFormat) {
					logrus.Errorf("Invalid file format. Given format: %v expectedFormat: %v", each, indodanaFormat)
					handler.OnErrorHandler("invalidFileError", channelName, err)
					errorOnContent = true
					break firstLoop
				}
				for i := 0; i < len(each)-1; i++ {
					if each[i] != indodanaFormat[i] {
						logrus.Errorf("Invalid file format. Given format: %v expectedFormat: %v", each, indodanaFormat)
						handler.OnErrorHandler("invalidFileError", channelName, err)
						errorOnContent = true
						break firstLoop
					}
				}
			} else {
				countBefore++
			}
			content = append(content, each)
		}

		if errorOnContent {
			logrus.Errorf("Got error on file: %v . Skipping this file", file.Name())
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		outputDir := "./tmp/after/" + channelName

		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}
		arrName := strings.Split(localFileBefore.Name(), "/")

		newFilename := strings.ReplaceAll(arrName[len(arrName)-1], "_yokke-ptp", "")
		newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

		newFilename = newFilename + ".csv"
		localFileAfter := outputDir + "/" + newFilename
		newFile, err := os.Create(localFileAfter)
		if err != nil {
			logrus.Errorf("Error when create file %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		writer := csv.NewWriter(newFile)
		writer.Comma = ';'

		for _, each := range content {
			if err := writer.Write(each); err != nil {
				logrus.Errorf("Failed to write file %v: %v", newFile.Name(), err)
				handler.OnErrorHandler("internalError", channelName, err)
				continue
			}
		}

		writer.Flush()

		fmt.Println("INDODANA file " + localFileBefore.Name() + " converted to ---->  " + newFilename + " successfully")

		dstFile, err := clientDest.Create(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Errorf("Failed to put file %v to sftp server. Err: %", newFilename, err.Error())
			handler.OnErrorHandler("directoryError", channelName, err)
			continue
		}

		defer dstFile.Close()

		if _, err := newFile.Seek(0, 0); err != nil {
			logrus.Errorf("Failed to seek file %v: %v", newFile.Name(), err)
			handler.OnErrorHandler("internalError", channelName, err)
			continue
		}

		_, err = io.Copy(dstFile, newFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %v to sftp server. Err: %v", newFile.Name(), err.Error())
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		// read again to count row after converted
		convertedFile, err := clientDest.Open(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Fatalf("Failed to read file (converted) %s: %v", newFilename, err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
		}

		defer convertedFile.Close()

		reader := csv.NewReader(convertedFile)
		reader.Comma = ';'
		convertedRecords, err := reader.ReadAll()
		if err != nil {
			logrus.Errorf("Failed to read csv: %v", err)
			handler.OnErrorHandler("invalidFileError", channelName, err)
			continue
		}

		var countAfter int
		countAfter = len(convertedRecords) - 1

		logrus.Printf("Count before: %d", countBefore)
		logrus.Printf("Count after: %d", countAfter)

		logrus.Printf("Success converting file")

		handler.OnSuccessHandler("", "Indodana", countBefore, countAfter)

		err = os.Remove(localPathBefore)
		if err != nil {
			logrus.Errorf("Failed to remove local file %v", err)
		}
		err = os.Remove(localFileAfter)
		if err != nil {
			logrus.Errorf("Failed to remove local file %v", err)
		}

		backupPath := handler.Config.Indodana.BackupPath + "/" + file.Name()
		err = client.Rename(remoteFileSourcePath, backupPath)
		if err != nil {
			logrus.Errorf("Failed to backup remote file %v to %v . Err: %v", remoteFileSourcePath, backupPath, err)
		}

	}

}

func (handler *Handler) OnErrorHandler(reason string, channelName string, err error) {
	message := gomail.NewMessage()
	message.SetHeader("From", handler.Config.Smtp.From)
	message.SetHeader("To", handler.Config.MailReceivers...)
	now := time.Now().Format("2006-01-02 15:04:05")
	subject := "Proses Konversi Excel ke CSV - " + channelName + " " + now

	asset := handler.Assets.Templates[mail.NotifConverted]
	if asset == nil {
		logrus.Errorf("Asset not found or invalid error type")
		return
	}

	templateData := struct {
		Subject            string
		AvailableStatus    string
		ConversionStatus   string
		DeliveryStatus     string
		RowBefore          string
		RowAfter           string
		ConditionalMessage string
	}{
		Subject:            subject,
		ConditionalMessage: reasonsMap[reason],
	}

	bBody := new(bytes.Buffer)
	if err := asset.Execute(bBody, templateData); err != nil {
		logrus.Errorf("Error parsing template : %v", err)
	}
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", bBody.String())

	err = handler.MailSender.DialAndSend(message)
	if err != nil {
		logrus.Errorf("Error sending email: %v", err)
	}
}

func (handler *Handler) OnSuccessHandler(reason string, channelName string, rowBefore, rowAfter int) {
	message := gomail.NewMessage()
	message.SetHeader("From", handler.Config.Smtp.From)
	message.SetHeader("To", handler.Config.MailReceivers...)
	now := time.Now().Format("2006-01-02 15:04:05")
	subject := "[Berhasil] Proses Konversi Excel ke CSV - " + channelName + " " + now

	asset := handler.Assets.Templates[mail.NotifConverted]
	if asset == nil {
		logrus.Errorf("Asset not found or invalid error type")
		return
	}

	templateData := struct {
		Subject            string
		AvailableStatus    string
		ConversionStatus   string
		DeliveryStatus     string
		RowBefore          string
		RowAfter           string
		ConditionalMessage string
	}{
		Subject:            subject,
		ConditionalMessage: "Tidak ada perubahan jumlah data. Silahkan verifikasi isi file jika diperlukan",
		AvailableStatus:    "OK",
		ConversionStatus:   "OK",
		DeliveryStatus:     "OK",
		RowBefore:          strconv.Itoa(rowBefore),
		RowAfter:           strconv.Itoa(rowAfter),
	}

	bBody := new(bytes.Buffer)
	if err := asset.Execute(bBody, templateData); err != nil {
		logrus.Errorf("Error parsing template : %v", err)
	}
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", bBody.String())

	err := handler.MailSender.DialAndSend(message)
	if err != nil {
		logrus.Errorf("Error sending email: %v", err)
	}
}

func (handler *Handler) CreateClient(sftpConfig config.Sftp) (conn *ssh.Client, cl *sftp.Client, e error) {
	var err error
	sshConfig := &ssh.ClientConfig{
		User: sftpConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(sftpConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err = ssh.Dial("tcp", sftpConfig.Host+":"+strconv.Itoa(sftpConfig.Port), sshConfig)
	if err != nil {
		logrus.Fatalf("Failed to dial: %v", err)
		return nil, nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		logrus.Fatalf("Failed to create SFTP client: %v", err)
		return conn, nil, err
	}

	return conn, client, err
}
