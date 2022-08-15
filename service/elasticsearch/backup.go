package elasticsearch

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/swisscom/backman/config"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/s3"
	"github.com/swisscom/backman/service/util"
	"github.com/swisscom/backman/state"
)

var esMutex = &sync.Mutex{}

func Backup(ctx context.Context, s3 *s3.Client, service util.Service, binding *cfenv.Service, filename string) (bool, string, error) {
	state.BackupQueue(service)

	// lock global elasticsearch mutex, only 1 backup/restore operation of this service-type is allowed to run in parallel
	esMutex.Lock()
	defer esMutex.Unlock()

	state.BackupStart(service)

	host, _ := binding.CredentialString("host")
	username, _ := binding.CredentialString("full_access_username")
	password, _ := binding.CredentialString("full_access_password")
	if len(username) == 0 {
		username, _ = binding.CredentialString("username")
	}
	if len(password) == 0 {
		password, _ = binding.CredentialString("password")
	}

	u, _ := url.Parse(host)

	// START CUSTOMIZING - escape username and password-------------------------------------
	connectstring := fmt.Sprintf("%s://%s:%s@%s", u.Scheme, url.QueryEscape(username), url.QueryEscape(password), u.Host)
	// STOP CUSTOMIZING -----------------------------------------------------------------------------

	// START CUSTOMIZING - Only dump the indexes from yesterday -------------------------------------
	backupCleaner := config.Get().BackupCleaner

	referenceTime := time.Now()
	outdatedEndTime := referenceTime.Add(time.Duration(backupCleaner.OutdatedEndHours*-1) * time.Hour)
	outdatedStartTime := referenceTime.Add(time.Duration(backupCleaner.OutdatedStartHours*-1) * time.Hour)
	referenceTimestamp := referenceTime.Format("20060102150405")
	yesterdayDateHour := referenceTime.Add(-24 * time.Hour).Format("2006.01.02-15")
	filename = fmt.Sprintf("index-esc-x-%s_BACKUPTIME-%s.gz", yesterdayDateHour, referenceTimestamp)
	indexDateHour := yesterdayDateHour

	if backupCleaner.Enabled {
		if backupCleaner.Type == "cleaner" {
			gapFound := false
			endReached := false
			maxLoops := 1000

			//get regular dates in regular files
			endDateHour := referenceTime
			objectPath := fmt.Sprintf("%s/%s/%s/regular-", service.Label, service.Name, backupCleaner.Group)
			objects, err := s3.List(objectPath)
			if err != nil {
				log.Warnf("backup cleaner - could not list S3 regular objects: %v", err)
			}
			// eval min date of all regular files
			for _, obj := range objects {
				tempDateHour := GetCleanerDate(s3, obj.Key, referenceTime)
				if tempDateHour.Before(endDateHour) && tempDateHour.After(outdatedEndTime) {
					endDateHour = tempDateHour
				}
			}
			// set default end date
			if endDateHour.Equal(referenceTime) {
				endDateHour = referenceTime.Add(time.Duration(backupCleaner.DefaultEndHours*-1) * time.Hour)
			}

			// eval start date
			startDateHour := referenceTime
			objectPath = fmt.Sprintf("%s/%s/%s/%s-%s", service.Label, service.Name, backupCleaner.Group, backupCleaner.Type, backupCleaner.Instance)
			tempDateHour := GetCleanerDate(s3, objectPath, referenceTime)
			if tempDateHour.After(outdatedStartTime) {
				startDateHour = tempDateHour
			}

			// set default start date
			if startDateHour.Equal(referenceTime) {
				startDateHour = referenceTime.Add(time.Duration(backupCleaner.DefaultStartHours*-1) * time.Hour)
			}

			// Loop per day and searching for first gap
			workingDateHour := startDateHour
			for i := 0; i < maxLoops && !gapFound && !endReached; i++ {
				workingDateDayString := workingDateHour.Format("2006.01.02")

				//get ES list of current day
				url := fmt.Sprintf("%s/index-esc-*-%s-*", host, workingDateDayString)
				indexes, err := GetIndexes(url, username, password)
				if err != nil {
					log.Errorf("backup cleaner - could not list ES indexes : %v", err)
					return false, filename, err
				}

				// get S3 list of current day
				m := make(map[string]string)
				objectPath = fmt.Sprintf("%s/%s/index-esc-x-%s", service.Label, service.Name, workingDateDayString)
				objects, err = s3.List(objectPath)
				if err != nil {
					log.Errorf("backup cleaner - could not list S3 backup objects: %v", err)
					return false, filename, err
				}

				// check all S3 entries and store them in a map
				for _, obj := range objects {
					objFilename := filepath.Base(obj.Key)
					dateHourString := objFilename[12:25]
					_, err := time.Parse("2006.01.02-15", dateHourString)
					if err == nil {
						m[dateHourString] = objFilename
					}
				}

				// search gap
				newStartDateHour := startDateHour
				for _, dateHourString := range indexes {
					dateHour, err := time.Parse("2006.01.02-15", dateHourString)
					if err != nil {
						log.Errorf("backup cleaner - could not parse index date : %v", err)
						return false, filename, err
					}
					// stop searching if end date is reached
					if !endDateHour.After(dateHour) {
						endReached = true
						break
					}

					// check ES date pattern
					pattern := ".*"
					if backupCleaner.EsDateHourPattern != "" {
						pattern = backupCleaner.EsDateHourPattern
					}
					match, _ := regexp.MatchString(pattern, dateHourString)

					if !match {
						continue
					}

					//search for gap when current date is after or equal start date
					if !startDateHour.After(dateHour) {
						newStartDateHour = dateHour
						if _, s3found := m[dateHourString]; !s3found {
							log.Infof("backup cleaner - gap found at: %s", dateHourString)
							gapFound = true
							// adapt filename
							indexDateHour = dateHourString
							filename = fmt.Sprintf("index-esc-x-%s_BACKUPTIME-%s.gz", dateHourString, referenceTimestamp)
							break
						}
					}
				}
				// write current cleaner dateHour to S3 as the new starting date
				if newStartDateHour.After(startDateHour) {
					newStartDateHourString := newStartDateHour.Format("2006.01.02-15")
					objectPath := fmt.Sprintf("%s/%s/%s/%s-%s", service.Label, service.Name, backupCleaner.Group, backupCleaner.Type, backupCleaner.Instance)
					r := strings.NewReader(newStartDateHourString)
					err = s3.Upload(objectPath, r, -1)
					if err != nil {
						log.Warnf("backup cleaner - could not upload current cleaner backup state to S3: %v", err)
					}
				}
				workingDateHour = workingDateHour.Add(24 * time.Hour)
			}

			// do not backup
			if endReached {
				return true, filename, nil
			}
		}

		if backupCleaner.Type == "regular" {
			objectPath := fmt.Sprintf("%s/%s/%s/%s-%s", service.Label, service.Name, backupCleaner.Group, backupCleaner.Type, backupCleaner.Instance)
			r := strings.NewReader(indexDateHour)
			err := s3.Upload(objectPath, r, -1)
			if err != nil {
				log.Warnf("backup cleaner - could not upload current regular backup state to S3: %v", err)
			}
		}
	}

	// STOP CUSTOMIZING -----------------------------------------------------------------------------

	// prepare elasticdump command
	var command []string
	command = append(command, "elasticdump")
	// START CUSTOMIZING - Only dump the indexes from yesterday -------------------------------------
	command = append(command, fmt.Sprintf("--input=%s/index-esc-*-%s", connectstring, indexDateHour))
	// command = append(command, fmt.Sprintf("--input=%s", connectstring))
	// STOP CUSTOMIZING -----------------------------------------------------------------------------
	command = append(command, "--output=$")

	log.Debugf("executing elasticsearch backup command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// capture stdout to pass to gzipping buffer
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for elasticdump: %v", err)
		state.BackupFailure(service)
		return false, filename, err
	}
	defer outPipe.Close()

	var uploadWait sync.WaitGroup
	uploadCtx, uploadCancel := context.WithCancel(context.Background()) // allows upload to be cancelable, in case backup times out
	defer uploadCancel()                                                // cancel upload in case Backup() exits before uploadWait is done

	// start upload in background, streaming output onto S3
	uploadWait.Add(1)
	go func() {
		defer uploadWait.Done()

		// gzipping stdout
		pr, pw := io.Pipe()
		gw := gzip.NewWriter(pw)
		gw.Name = strings.TrimSuffix(filename, ".gz")
		gw.ModTime = time.Now()
		go func() {
			_, _ = io.Copy(gw, bufio.NewReader(outPipe))
			if err := gw.Flush(); err != nil {
				log.Errorf("%v", err)
			}
			if err := gw.Close(); err != nil {
				log.Errorf("%v", err)
			}
			if err := pw.Close(); err != nil {
				log.Errorf("%v", err)
			}
		}()

		objectPath := fmt.Sprintf("%s/%s/%s", service.Label, service.Name, filename)
		err = s3.UploadWithContext(uploadCtx, objectPath, pr, -1)
		if err != nil {
			log.Errorf("could not upload service backup [%s] to S3: %v", service.Name, err)
			state.BackupFailure(service)
		}
	}()
	time.Sleep(2 * time.Second) // wait for upload goroutine to be ready

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run elasticdump: %v", err)
		state.BackupFailure(service)
		return false, filename, err
	}

	if err := cmd.Wait(); err != nil {
		state.BackupFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return false, filename, fmt.Errorf("elasticdump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return false, filename, fmt.Errorf("elasticdump: %v", err)
	}

	uploadWait.Wait() // wait for upload to have finished
	if err == nil {
		state.BackupSuccess(service)
	}
	return false, filename, err
}

// START CUSTOMIZING - Add function for single index backup --------------------------------------
func BackupSingle(ctx context.Context, s3 *s3.Client, service util.Service, binding *cfenv.Service, filename string, index string) error {
	state.BackupQueue(service)

	// lock global elasticsearch mutex, only 1 backup/restore operation of this service-type is allowed to run in parallel
	esMutex.Lock()
	defer esMutex.Unlock()

	state.BackupStart(service)

	host, _ := binding.CredentialString("host")
	username, _ := binding.CredentialString("full_access_username")
	password, _ := binding.CredentialString("full_access_password")
	if len(username) == 0 {
		username, _ = binding.CredentialString("username")
	}
	if len(password) == 0 {
		password, _ = binding.CredentialString("password")
	}

	u, _ := url.Parse(host)
	connectstring := fmt.Sprintf("%s://%s:%s@%s", u.Scheme, url.QueryEscape(username), url.QueryEscape(password), u.Host)

	// prepare elasticdump command
	var command []string
	command = append(command, "elasticdump")
	command = append(command, "--debug true")
	command = append(command, fmt.Sprintf("--input=%s/%s", connectstring, index))
	command = append(command, "--output=$")

	log.Debugf("executing elasticsearch backup command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// capture stdout to pass to gzipping buffer
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for elasticdump: %v", err)
		state.BackupFailure(service)
		return err
	}
	defer outPipe.Close()

	var uploadWait sync.WaitGroup
	uploadCtx, uploadCancel := context.WithCancel(context.Background()) // allows upload to be cancelable, in case backup times out
	defer uploadCancel()                                                // cancel upload in case Backup() exits before uploadWait is done

	// start upload in background, streaming output onto S3
	uploadWait.Add(1)
	go func() {
		defer uploadWait.Done()

		// gzipping stdout
		pr, pw := io.Pipe()
		gw := gzip.NewWriter(pw)
		gw.Name = strings.TrimSuffix(filename, ".gz")
		gw.ModTime = time.Now()
		go func() {
			_, _ = io.Copy(gw, bufio.NewReader(outPipe))
			if err := gw.Flush(); err != nil {
				log.Errorf("%v", err)
			}
			if err := gw.Close(); err != nil {
				log.Errorf("%v", err)
			}
			if err := pw.Close(); err != nil {
				log.Errorf("%v", err)
			}
		}()

		objectPath := fmt.Sprintf("%s/%s/%s", service.Label, service.Name, filename)
		err = s3.UploadWithContext(uploadCtx, objectPath, pr, -1)
		if err != nil {
			log.Errorf("could not upload service backup [%s] to S3: %v", service.Name, err)
			state.BackupFailure(service)
		}
	}()
	time.Sleep(2 * time.Second) // wait for upload goroutine to be ready

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run elasticdump: %v", err)
		state.BackupFailure(service)
		return err
	}

	if err := cmd.Wait(); err != nil {
		state.BackupFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("elasticdump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return fmt.Errorf("elasticdump: %v", err)
	}

	uploadWait.Wait() // wait for upload to have finished
	if err == nil {
		state.BackupSuccess(service)
	}
	return err
}

func GetCleanerDate(s3 *s3.Client, filename string, defaultDate time.Time) time.Time {
	r, err := s3.Download(filename)
	if err != nil {
		return defaultDate
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, r)
	if err != nil {
		return defaultDate
	}

	dateHourString := buf.String()
	dateHour, err := time.Parse("2006.01.02-15", dateHourString)
	if err != nil {
		return defaultDate
	}
	return dateHour
}

func GetIndexes(url string, username string, password string) ([]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []string{}, err
	}

	req.SetBasicAuth(username, password)
	cli := &http.Client{}

	response, err := cli.Do(req)
	if response != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return []string{}, err
	}

	if response.StatusCode != 200 {
		return []string{}, errors.New("Request failed with status: " + response.Status)
	}

	//decode json
	var indexes map[string]json.RawMessage
	err = json.NewDecoder(response.Body).Decode(&indexes)
	if err != nil {
		return []string{}, err
	}

	//sort
	keys := make([]string, 0, len(indexes))
	for key := range indexes {
		keyDate := strings.SplitN(key, "-", 4)
		keys = append(keys, keyDate[3])
	}
	sort.Strings(keys)

	return keys, nil
}

// STOP CUSTOMIZING -----------------------------------------------------------------------------
