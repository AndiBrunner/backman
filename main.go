package main

import (
	"flag"

	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/router"
	"github.com/swisscom/backman/scheduler"
	"github.com/swisscom/backman/service"
	"github.com/swisscom/backman/service/util"
)

//go:generate swagger generate spec
func main() {
	log.Infoln("starting up backman ...")

	// init services
	service.Get()

	// check if an immediate backup/restore should run, in non-background mode. otherwise continue and start scheduler
	if runNow() {
		return
	}

	// schedule backups
	scheduler.RegisterBackups()

	// serve API & UI
	r := router.New()
	log.Fatalf("%v", r.Start())
}

func runNow() bool {
	serviceToBackup := flag.String("backup", "", "service to backup now")
	// START CUSTOMIZING - Add function for dedicated index backup --------------------------------------
	serviceToBackupSingle := flag.String("backupdedicated", "", "service to backup now")
	indexToBackup := flag.String("index", "", "index to use for backup restore")
	// STOP CUSTOMIZING -----------------------------------------------------------------------------
	serviceToRestore := flag.String("restore", "", "service to restore now")
	filenameToRestore := flag.String("filename", "", "filename to use for service restore")
	flag.Parse()

	// START CUSTOMIZING - Add function for single index backup --------------------------------------
        // check if backup for single index should run
        if len(*serviceToBackupSingle) > 0 && len(*indexToBackup) > 0 {
                log.Infof("backup for dedicated index flag provided with value [%s/%s], running backup now", *serviceToBackupSingle, *indexToBackup)

                // setting config to non-background mode, to avoid background goroutines during backups
                config.Get().Foreground = true

                // find service to backup
                var found bool
                for _, s := range service.Get().Services {
                        if s.Name == *serviceToBackupSingle {
                                // running backup
                                log.Infof("running service backup for [%s/%s/%s]", s.Label, s.Name, *indexToBackup)
                                if err := service.Get().BackupSingle(s, *indexToBackup); err != nil {
                                        log.Fatalf("service backup failed: %v", err)
                                }
                                found = true

                                // running S3 cleanup
                                if err := service.Get().RetentionCleanup(s); err != nil {
                                        log.Errorf("could not cleanup S3 storage for service [%s]: %v", s.Name, err)
                                }
                                break
                        }
                }
                if !found {
                        log.Fatalf("could not find any service named [%s]", *serviceToBackupSingle)
                }

                log.Infoln("backup successfully completed")
                return true
        }
	// STOP CUSTOMIZING -----------------------------------------------------------------------------

	// check if backup should run
	if len(*serviceToBackup) > 0 {
		log.Infof("backup flag provided with value [%s], running backup now", *serviceToBackup)

		// setting config to non-background mode, to avoid background goroutines during backups
		config.Get().Foreground = true

		// find service to backup
		var found bool
		for _, s := range service.Get().Services {
			if s.Name == *serviceToBackup {
				// running backup
				log.Infof("running service backup for [%s/%s]", s.Label, s.Name)
				if err := service.Get().Backup(s); err != nil {
					log.Fatalf("service backup failed: %v", err)
				}
				found = true

				// running S3 cleanup
				if err := service.Get().RetentionCleanup(s); err != nil {
					log.Errorf("could not cleanup S3 storage for service [%s]: %v", s.Name, err)
				}
				break
			}
		}
		if !found {
			log.Fatalf("could not find any service named [%s]", *serviceToBackup)
		}

		log.Infoln("backup successfully completed")
		return true
	}

	// check if restore should run
	if len(*serviceToRestore) > 0 && len(*filenameToRestore) > 0 {
		log.Infof("restore flags provided with values [%s/%s], running restore now", *serviceToRestore, *filenameToRestore)

		// find service to restore
		var found bool
		for _, s := range service.Get().Services {
			if s.Name == *serviceToRestore {
				// running restore
				log.Infof("running service restore for [%s/%s] with filename [%s]", s.Label, s.Name, *filenameToRestore)
				if err := service.Get().Restore(s, util.Service{}, *filenameToRestore); err != nil {
					log.Fatalf("service restore failed: %v", err)
				}
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("could not find any service named [%s]", *serviceToRestore)
		}

		log.Infoln("restore successfully completed")
		return true
	}

	return false
}
