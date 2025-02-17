# Notes

## Explaination

This program connects to an **SFTP (Secure File Transfer Protocol) server**, finds files that match a specific pattern (names starting with `oms_installer` and ending with `.tar.gz` or `.tgz`), and downloads them to a local directory.

### **Breaking It Down**

#### **1. Importing Required Packages**

At the top, the program imports some necessary **Go packages**:

```go
import (
 "fmt"         // For printing messages
 "io"          // Handles file input/output operations
 "os"          // Accesses environment variables & handles file system
 "strings"     // Helps with string operations
 "github.com/joho/godotenv"  // Reads values from a .env file
 "github.com/pkg/sftp"       // Allows interaction with an SFTP server
 "golang.org/x/crypto/ssh"   // Manages SSH (secure shell) connections
)
```

---

#### **2. Loading Configuration from a `.env` File**

The program reads server details (like hostname, username, password, etc.) from a **.env file**.
The `.env` file is a simple text file that contains key-value pairs (like `SFTP_HOSTNAME=example.com`).

```go
err := godotenv.Load()
if err != nil {
 fmt.Println("❌ Error loading .env file:", err)
 return
}
```

- `godotenv.Load()` reads the `.env` file and loads variables into memory.
- If the file is missing or has errors, it prints an error message and **stops execution** (`return`).

---

#### **3. Getting SFTP Connection Details from the Environment**

```go
sftpHost := os.Getenv("SFTP_HOSTNAME")
sftpPort := os.Getenv("SFTP_PORT")
sftpUser := os.Getenv("SFTP_USERNAME")
sftpPassword := os.Getenv("SFTP_PASSWORD")
remotePath := os.Getenv("SFTP_REMOTE_DIRECTORY")
localPath := os.Getenv("LOCAL_DIRECTORY")
```

This fetches required details from the `.env` file using `os.Getenv()`, which retrieves system environment variables.

---

#### **4. Setting Up SSH Connection**

Since SFTP runs over SSH, we need to establish an **SSH connection** first.

```go
config := &ssh.ClientConfig{
 User: sftpUser,
 Auth: []ssh.AuthMethod{
  ssh.Password(sftpPassword),
 },
 HostKeyCallback: ssh.InsecureIgnoreHostKey(),
}
```

- `ssh.ClientConfig` holds **username and password** to authenticate.
- `ssh.InsecureIgnoreHostKey()` disables strict host key checking (this is insecure but useful for testing).

Then, the program **tries to connect** using:

```go
sshConn, err := ssh.Dial("tcp", sftpHost+":"+sftpPort, config)
if err != nil {
 fmt.Println("❌ Failed to connect to SSH:", err)
 return
}
defer sshConn.Close()
```

- `ssh.Dial()` attempts to establish a **TCP connection** to the SFTP server.
- If it fails, an error message is printed, and the program **exits** (`return`).
- `defer sshConn.Close()` ensures that when the function ends, the SSH connection is closed.

---

#### **5. Creating an SFTP Client**

Now that SSH is connected, the program **creates an SFTP client**:

```go
sftpClient, err := sftp.NewClient(sshConn)
if err != nil {
 fmt.Println("❌ Failed to create SFTP client:", err)
 return
}
defer sftpClient.Close()
```

- `sftp.NewClient(sshConn)` creates a client for file transfers.
- If it fails, the program prints an error and **stops**.
- `defer sftpClient.Close()` ensures that the connection **closes automatically** when the function finishes.

---

#### **6. Ensuring the Local Directory Exists**

```go
if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
 fmt.Println("❌ Failed to create local directory:", err)
 return
}
```

- `os.MkdirAll(localPath, os.ModePerm)` creates the local directory if it **does not exist**.
- If it fails, the program prints an error and **exits**.

---

#### **7. Function to Download Files from the SFTP Server**

The function `downloadFiles()` is **recursive**, meaning it can **go inside subdirectories** if needed.

```go
var downloadFiles func(string) error
downloadFiles = func(remoteDir string) error {
 remoteFiles, err := sftpClient.ReadDir(remoteDir)
 if err != nil {
  return fmt.Errorf("❌ Failed to list remote directory: %v", err)
 }
```

- It reads **all files and directories** inside `remoteDir`.
- If it **fails** to list files, it returns an error message.

##### **Checking Each File**

```go
for _, file := range remoteFiles {
 remoteFilePath := remoteDir + "/" + file.Name()
 localFilePath := localPath + "/" + file.Name()
```

- Loops through each file in the **remote directory**.
- Constructs the full **remote and local paths** for the file.

##### **Handling Subdirectories**

```go
if file.IsDir() {
 if err := downloadFiles(remoteFilePath); err != nil {
  fmt.Println(err)
 }
}
```

- If the file is a **folder**, it calls `downloadFiles()` **again** (recursive call).

##### **Downloading Matching Files**

```go
} else if strings.HasPrefix(strings.ToLower(file.Name()), "oms_installer") && (strings.HasSuffix(file.Name(), ".tar.gz") || strings.HasSuffix(file.Name(), ".tgz")) {
```

- Checks if the filename **starts with** `oms_installer` and **ends with** `.tar.gz` or `.tgz`.

##### **Skipping Already Downloaded Files**

```go
if _, err := os.Stat(localFilePath); err == nil {
 fmt.Println("⚠️  File already exists, skipping download:", localFilePath)
 continue
}
```

- If the file **already exists locally**, it prints a message and **skips downloading**.

##### **Opening the Remote File**

```go
remoteFile, err := sftpClient.Open(remoteFilePath)
if err != nil {
 fmt.Println("❌ Failed to open remote file:", err)
 continue
}
defer remoteFile.Close()
```

- Opens the **remote file** for reading.
- If it **fails**, an error is printed, and it **moves to the next file**.

##### **Creating the Local File**

```go
localFile, err := os.Create(localFilePath)
if err != nil {
 fmt.Println("❌ Failed to create local file:", err)
 continue
}
defer localFile.Close()
```

- Creates a **new local file** to save the downloaded content.
- If it **fails**, it prints an error and **skips the file**.

##### **Copying Data from Remote to Local**

```go
fmt.Println("⬇️  Downloading file:", file.Name())
_, err = io.Copy(localFile, remoteFile)
if err != nil {
 fmt.Println("❌ Error copying file:", err)
} else {
 fmt.Println("✅ Download completed:", localFilePath)
}
```

- **Copies the file contents** from the remote server to the local system.
- If successful, it prints `"✅ Download completed"`; otherwise, it prints an error.

---

#### **8. Starting the Download Process**

```go
if err := downloadFiles(remotePath); err != nil {
 fmt.Println(err)
}
```

- Calls `downloadFiles(remotePath)`, which starts the recursive download process.

---

### **Final Thoughts**

- ✅ Reads configuration from a `.env` file
- ✅ Connects to an **SFTP server**
- ✅ **Finds and downloads** matching `.tar.gz` or `.tgz` files
- ✅ **Handles subdirectories recursively**
- ✅ **Skips already downloaded files**
