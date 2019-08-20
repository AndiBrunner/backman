package mysql

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-community/go-cfenv"
	"gitlab.swisscloud.io/appc-cf-core/appcloud-backman-app/log"
	"gitlab.swisscloud.io/appc-cf-core/appcloud-backman-app/s3"
)

var mysqlMutex = &sync.Mutex{}

func Backup(ctx context.Context, s3 *s3.Client, binding *cfenv.Service, filename string) error {
	// lock global mysql mutex, only 1 backup of this service-type is allowed to run in parallel
	// to avoid issues with setting MYSQL* environment variables and memory consumption
	mysqlMutex.Lock()
	defer mysqlMutex.Unlock()

	host, _ := binding.CredentialString("host")
	port, _ := binding.CredentialString("port")
	database, _ := binding.CredentialString("database")
	username, _ := binding.CredentialString("username")
	password, _ := binding.CredentialString("password")

	os.Setenv("MYSQL_PWD", password)

	// prepare mysqldump command
	var command []string
	command = append(command, "mysqldump")
	if len(database) > 0 {
		command = append(command, "--databases")
		command = append(command, database)
	} else {
		command = append(command, "--all-databases")
	}
	command = append(command, "--single-transaction")
	command = append(command, "--quick")
	command = append(command, "-h")
	command = append(command, host)
	command = append(command, "-P")
	command = append(command, port)
	command = append(command, "-u")
	command = append(command, username)

	log.Debugf("executing mysql backup command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// capture stdout to pass to gzipping buffer
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for mysqldump: %v", err)
		return err
	}

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run mysqldump: %v", err)
		return err
	}

	var uploadWait sync.WaitGroup
	uploadCtx, uploadCancel := context.WithCancel(context.Background()) // allows upload to be cancelable, in case backup times out
	defer uploadCancel()                                                // cancel upload in case Backup() exits before uploadWait is done
	go func() {
		uploadWait.Add(1)
		defer uploadWait.Done()

		// gzipping stdout
		pr, pw := io.Pipe()
		gw := gzip.NewWriter(pw)
		gw.Name = filename
		gw.ModTime = time.Now()
		go func() {
			defer pw.Close()
			defer gw.Close()
			_, _ = io.Copy(gw, outPipe)
		}()

		objectPath := fmt.Sprintf("%s/%s/%s", binding.Label, binding.Name, filename)
		err = s3.UploadWithContext(uploadCtx, objectPath, pr, -1)
		if err != nil {
			log.Errorf("could not upload service backup [%s] to S3: %v", binding.Name, err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mysqldump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return fmt.Errorf("mysqldump: %v", err)
	}

	uploadWait.Wait() // wait for upload to have finished
	return err
}
