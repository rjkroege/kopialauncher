package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	SUCCESSS = iota
	ERR_NO_SNAPSHOT
	ERR_NO_LIST
	ERR_NO_SNAP_IN_LIST
	ERR_NO_DIR
	ERR_NO_MOUNT
	ERR_NO_KOPIA
	ERR_NO_UNMOUNT
	ERR_BAD_LOGGING
	ERR_BAD_ENV
	SNAPSHOT     = "/tmp/snapshot"
	TMDATELAYOUT = "2006-01-02-150405"
)

// setupLogging configures the Go logger to write log messages to the "standard"
// place on MacOS.
// TODO(rjk): Figure out if I have to roll these myself or if MacOS will roll them for
// me.
// TODO(rjk): Generalize to more than Darwin
func setupLogging() error {
	// Based on how Kopia organizes its logs.
	logFileName := fmt.Sprintf("%v-%v-%v%v", "kopialauncher", time.Now().Format("20060102-150405"), os.Getpid(), ".log")

	logDir := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "kopialauncher")

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

// Dump the log to the console instead with this flag.
var consolelog = flag.Bool("log", false, "log to console")

func main() {
	flag.Parse()

	if !*consolelog {
		if err := setupLogging(); err != nil {
			log.Println("kopialauncher:", err)
			os.Exit(ERR_BAD_LOGGING)
		}
	}

	// Demonstrate that we log things.
	log.Println("Running kopialauncher")

	// 0. Verify that we have a credentials file so that Kopia can access GCS.
	gac := os.ExpandEnv("$GOOGLE_APPLICATION_CREDENTIALS")
	if gac == "" {
		log.Println("GOOGLE_APPLICATION_CREDENTIALS not defined")
		os.Exit(ERR_BAD_ENV)
	}
	if fi, err := os.Stat(gac); err != nil || !fi.Mode().IsRegular() {
		log.Println("GOOGLE_APPLICATION_CREDENTIALS needs to point at an accessible credentials file")
		os.Exit(ERR_BAD_ENV)
	}

	// 1. APFS snapshot
	tmutilsnapcmd := exec.Command("/usr/bin/tmutil", "localsnapshot")
	if err := tmutilsnapcmd.Run(); err != nil {
		log.Println("kopialauncher can't run tmutil localsnapshot", err)
		os.Exit(ERR_NO_SNAPSHOT)
	}

	// TODO(rjk): stop here if there is no network access.

	// 2. Mount and find last one
	tmutillistcmd := exec.Command("/usr/bin/tmutil", "listlocalsnapshots", "/System/Volumes/Data")
	out, err := tmutillistcmd.Output()
	if err != nil {
		log.Println("kopialauncher can't run tmutil listlocalsnapshots:", err, "output:", string(out))
		os.Exit(ERR_NO_LIST)
	}

	// The list of snapshots are sorted. timemachine isn't the only source of
	// snapshots in the tmutil listlocalsnapshots output. We need to pick the
	// lastest snapshot that was made with tmutil.
	snapshots := bytes.Split(out, []byte("\n"))

	// Apple inverted the order of the tmutil list output in 11.1 beta 2.
	// Parse it properly.
	lt := time.Time{}
	for _, s := range snapshots {
		if bytes.HasPrefix(s, []byte("com.apple.TimeMachine.")) && bytes.HasSuffix(s, []byte(".local")) {
			datestring := string(s[len("com.apple.TimeMachine.") : len(s)-len(".local")])
			// log.Println("> current datestring", datestring)
			if t, err := time.Parse(TMDATELAYOUT, datestring); err == nil && t.After(lt) {
				lt = t
			}
		}
	}

	lastsnap := "com.apple.TimeMachine." + lt.Format(TMDATELAYOUT) + ".local"
	log.Println("last snapshot: ", lastsnap)

	if err := os.MkdirAll(SNAPSHOT, 0700); err != nil {
		log.Println("kopialauncher can't make: ", SNAPSHOT, "because:", err)
		os.Exit(ERR_NO_DIR)
	}

	// We might have already mounted something. Try unmounting.
	// This should fail if we're already doing a backup?
	unmount := exec.Command("/usr/sbin/diskutil", "unmount", SNAPSHOT)
	if err := unmount.Run(); err != nil {
		log.Println("kopialauncher can't unmount. Probably because we're not mounted", err)
	}

	mount := exec.Command("/sbin/mount", "-t", "apfs", "-r", "-o", "-s="+lastsnap, "/System/Volumes/Data", SNAPSHOT)
	if err := mount.Run(); err != nil {
		log.Println("kopialauncher can't mount", err)
		os.Exit(ERR_NO_MOUNT)
	}

	// 3. Run Kopia proper
	log.Println("About to run kopia")
	kopia := exec.Command("/usr/local/bin/kopia", "snapshot", "create", filepath.Join(SNAPSHOT, "/Users/rjkroege"))
	// TODO(rjk): Stop collecting spew when I'm confidant that this works.
	spew, err := kopia.CombinedOutput()
	if err != nil {
		log.Println("kopialauncher can't run kopia", err, "spew:", string(spew))
		os.Exit(ERR_NO_KOPIA)
	}
	log.Println("Finished running kopia without errors, spew discarded")

	// Try unmounting.
	unmount = exec.Command("/usr/sbin/diskutil", "unmount", SNAPSHOT)
	if err := unmount.Run(); err != nil {
		log.Println("kopialauncher can't unmount", SNAPSHOT, "because:", err)
		os.Exit(ERR_NO_UNMOUNT)
	}
	log.Println("All done")

	// 4. APFS snapshot unmount
	os.Exit(SUCCESSS)
}
