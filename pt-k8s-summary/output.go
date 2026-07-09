package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

const esc = "\x1b"

func printWroteReport(outPath string, nodeCount, pxcCount, pxcFiles, backupCount, backupFiles, psCount, psFiles, psBackupCount, psBackupFiles, pgCount, pgFiles, pgBackupCount, pgBackupFiles int) {
	abs, err := filepath.Abs(outPath)
	if err != nil {
		abs = outPath
	}
	summary := fmt.Sprintf("(%d nodes, %d PXC cluster(s) from %d file(s), %d backup(s) from %d file(s), %d PS cluster(s) from %d file(s), %d PS backup(s) from %d file(s), %d PG cluster(s) from %d file(s), %d PG backup(s) from %d file(s))",
		nodeCount, pxcCount, pxcFiles, backupCount, backupFiles, psCount, psFiles, psBackupCount, psBackupFiles, pgCount, pgFiles, pgBackupCount, pgBackupFiles)

	pathOut := abs
	if stdoutIsTTY() {
		pathOut = terminalFileLink(abs)
	}
	fmt.Printf("Wrote %s %s\n", pathOut, summary)
}

func reportFileURL(absPath string) string {
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}
	return u.String()
}

func terminalFileLink(absPath string) string {
	return terminalHyperlink(reportFileURL(absPath), absPath)
}

func terminalHyperlink(linkURL, label string) string {
	return fmt.Sprintf("%s]8;;%s%s\\%s%s]8;;%s\\", esc, linkURL, esc, label, esc, esc)
}

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
