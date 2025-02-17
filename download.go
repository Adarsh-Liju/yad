package main

import (
	"database/sql"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"sync"
	_ "github.com/go-sql-driver/mysql"
)

type OmsArchive struct {
	OMSID          int    `json:"oms_id"`
	OMSFilename    string `json:"oms_filename"`
	OMSFilepath    string `json:"oms_filepath"`
	OMSMainVersion string `json:"oms_main_version"`
	IsDownloaded   bool   `json:"is_downloaded"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("❌ Error loading .env file:", err)
		return
	}

	// Get SFTP and database credentials from environment variables
	sftpHost := os.Getenv("SFTP_HOSTNAME")
	sftpPort := os.Getenv("SFTP_PORT")
	sftpUser := os.Getenv("SFTP_USERNAME")
	sftpPassword := os.Getenv("SFTP_PASSWORD")
	localPath := os.Getenv("LOCAL_DIRECTORY")
	dbConnStr := os.Getenv("MYSQL_DSN") // Example: "user:password@tcp(127.0.0.1:3306)/dbname"

	// Connect to MySQL database
	db, err := sql.Open("mysql", dbConnStr)
	if err != nil {
		fmt.Println("❌ Error connecting to the database:", err)
		return
	}
	defer db.Close()

	// Fetch files from the database
	rows, err := db.Query("SELECT oms_id, oms_filename, oms_filepath, oms_main_version, is_downloaded FROM oms_archives WHERE is_downloaded = 0")
	if err != nil {
		fmt.Println("❌ Error querying database:", err)
		return
	}
	defer rows.Close()

	var archives []OmsArchive
	for rows.Next() {
		var archive OmsArchive
		if err := rows.Scan(&archive.OMSID, &archive.OMSFilename, &archive.OMSFilepath, &archive.OMSMainVersion, &archive.IsDownloaded); err != nil {
			fmt.Println("❌ Error scanning row:", err)
			continue
		}
		archives = append(archives, archive)
	}

	// Establish SSH connection
	config := &ssh.ClientConfig{
		User: sftpUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sftpPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshConn, err := ssh.Dial("tcp", sftpHost+":"+sftpPort, config)
	if err != nil {
		fmt.Println("❌ Failed to connect to SSH:", err)
		return
	}
	defer sshConn.Close()

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		fmt.Println("❌ Failed to create SFTP client:", err)
		return
	}
	defer sftpClient.Close()

	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		fmt.Println("❌ Failed to create local directory:", err)
		return
	}

	// Download files concurrently
	const maxConcurrentDownloads = 5
	sem := make(chan struct{}, maxConcurrentDownloads)
	var wg sync.WaitGroup

	for _, archive := range archives {
		wg.Add(1)
		sem <- struct{}{} // Limit concurrency

		go func(archive OmsArchive) {
			defer wg.Done()
			defer func() { <-sem }()

			remoteFilePath := archive.OMSFilepath
			localFilePath := localPath + "/" + archive.OMSFilename

			if _, err := os.Stat(localFilePath); err == nil {
				fmt.Println("⚠️  File already exists, skipping:", localFilePath)
				return
			}

			fmt.Println("⬇️  Starting download:", archive.OMSFilename)
			remoteFile, err := sftpClient.Open(remoteFilePath)
			if err != nil {
				fmt.Println("❌ Failed to open remote file:", err)
				return
			}
			defer remoteFile.Close()

			localFile, err := os.Create(localFilePath)
			if err != nil {
				fmt.Println("❌ Failed to create local file:", err)
				return
			}
			defer localFile.Close()

			if _, err := io.Copy(localFile, remoteFile); err != nil {
				fmt.Println("❌ Error copying file:", err)
				return
			}

			fmt.Println("✅ Downloaded:", localFilePath)

			// Mark file as downloaded in database
			_, err = db.Exec("UPDATE oms_archives SET is_downloaded = 1 WHERE oms_id = ?", archive.OMSID)
			if err != nil {
				fmt.Println("❌ Error updating database:", err)
			} else {
				fmt.Println("✅ Database updated for:", archive.OMSFilename)
			}
		}(archive)
	}

	wg.Wait()
	fmt.Println("🎉 All downloads complete!")
}
