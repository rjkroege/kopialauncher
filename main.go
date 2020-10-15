package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"os"
	"path/filepath"
	"time"
	"os/user"
	"io/ioutil"
)

const (
	SUCCESSS = iota
	ERR_NO_SNAPSHOT
	ERR_NO_LIST 
	ERR_NO_SPLIT_LIST 
	ERR_NO_DIR 
	ERR_NO_MOUNT 
	ERR_NO_KOPIA 
	ERR_NO_UNMOUNT
	ERR_BAD_LOGGING
	SNAPSHOT = "/tmp/snapshot"
)

func setupLogging() error {
	logFileName := fmt.Sprintf("%v-%v-%v%v", "kopialauncher", time.Now().Format("20060102-150405"), os.Getpid(),  ".log")

	// TODO(rjk): Consider generalizing for more than darwin
	logDir :=  filepath.Join(os.Getenv("HOME"), "Library", "Logs", "kopialauncher" )

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("can't make log directory %s: %v", logDir, err)
	}

	path := filepath.Join(logDir, logFileName)
	fd, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("can't create log file %s: %v", path, err)
	}		
	
	log.SetOutput(fd)
	return nil	
}

var consolelog = flag.Bool("log", false, "log to console")

func main() {
	flag.Parse()
	
	if !*consolelog {
		if err := setupLogging(); err != nil {
			log.Println("kopialauncher:", err)
			os.Exit(ERR_BAD_LOGGING);
		}
	}

	// Test errors..
	log.Println("Running kopialauncher")

	// Test how Oauth library fetches creds.
	// this is right.
	log.Println("fetch dir", guessUnixHomeDir())
	credfile := filepath.Join(guessUnixHomeDir(), ".config", "gcloud", "application_default_credentials.json")
	bah, err := ioutil.ReadFile(credfile)
	if err != nil {
		log.Println("can't read credfile", err)
	}
	log.Println("creds:", string(bah))
// ---------- essay ----------

/*
so, I don't really know how it successfully gets credentials when running normally.
So, step one is to figure out kopia finds creds when running normally...

There's something about being invoked by launchagent

Yup. 

; env | grep -i cred
GOOGLE_APPLICATION_CREDENTIALS=/Users/rjkroege/.ssh/gubaidulina.json

No environment.
So, shove this into the env?
*/

// ------------------------
	

	// 1. APFS snapshot

	tmutilsnapcmd := exec.Command("/usr/bin/tmutil", "localsnapshot")
	if err := tmutilsnapcmd.Run(); err != nil {
		log.Println("kopialauncher can't run tmutil localsnapshot", err)
		os.Exit(ERR_NO_SNAPSHOT);
	}

// TODO(rjk): stop here if there is no network access or on battery power.

	// 2. Mount and find last one
	tmutillistcmd := exec.Command("/usr/bin/tmutil", "listlocalsnapshots", "/System/Volumes/Data")
	out, err := tmutillistcmd.Output()
	if err != nil {
		log.Println("kopialauncher can't run tmutil listlocalsnapshots:", err, "output:", string(out))
		os.Exit(ERR_NO_LIST);
	}

	snapshots := bytes.Split(out, []byte("\n"))
	if len(snapshots) < 2 {
		log.Println("kopialauncher no splittable snapshot list?",  "output:", string(out))
		os.Exit(ERR_NO_SPLIT_LIST);
	}

	// Last split result is an empty line.
	lastsnap := string(snapshots[len(snapshots)-2])
	log.Println("last snapshot: ", lastsnap)


	if err := os.MkdirAll(SNAPSHOT, 0700); err != nil {
		log.Println("kopialauncher can't make: ", SNAPSHOT , "because:", err)
		os.Exit(ERR_NO_DIR);
	}
	
	// We might have already mounted something. Try unmounting.
	// This should fail if we're already doing a backup?
	unmount := exec.Command("/usr/sbin/diskutil", "unmount", SNAPSHOT )
	if err := unmount.Run(); err != nil {
		log.Println("kopialauncher can't unmount. Probably because we're not mounted", err)
	}

	mount := exec.Command("/sbin/mount", "-t", "apfs", "-r", "-o", "-s=" + lastsnap, "/System/Volumes/Data", SNAPSHOT)
	if err := mount.Run(); err != nil {
		log.Println("kopialauncher can't mount", err)
		os.Exit(ERR_NO_MOUNT);
	}

	// 3. Run Kopia proper
	// TODO(rjk): get the homedir better-like
	// How do I know the home directory in a launch script?
	log.Println("About to run kopia")
	kopia := exec.Command("/usr/local/bin/kopia", "snapshot", "create", filepath.Join( SNAPSHOT, "/Users/rjkroege"))
	// Env should come from the plist file?
	kopia.Env = append(kopia.Env, "GOOGLE_APPLICATION_CREDENTIALS=/Users/rjkroege/.ssh/gubaidulina.json", "HOME=/Users/rjkroege")
	spew, err := kopia.CombinedOutput()
	if err := kopia.Run(); err != nil {
		log.Println("kopialauncher can't run kopia", err, "spew:", string(spew))
		os.Exit(ERR_NO_KOPIA);
	}
	log.Println("Finished running kopia")

	// Try unmounting.
	unmount = exec.Command("/usr/sbin/diskutil", "unmount", SNAPSHOT )
	if err := unmount.Run(); err != nil {
		log.Println("kopialauncher can't unmount" , SNAPSHOT, "because:" , err)
		os.Exit(ERR_NO_UNMOUNT);
	}

	// 4. APFS snapshot unmount
	os.Exit(SUCCESSS)
}



// --------- debugging -------------
func guessUnixHomeDir() string {
	// Prefer $HOME over user.Current due to glibc bug: golang.org/issue/13470
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	// Else, fall back to user.Current:
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return ""
}