package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// ensure that an added file shows up in docker diff
func TestDiffFilenameShownInOutput(t *testing.T) {
	containerCmd := `echo foo > /root/bar`
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", containerCmd)
	cid, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to start the container: %v", err))

	cleanCID := stripTrailingCharacters(cid)

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err := runCommandWithOutput(diffCmd)
	errorOut(err, t, fmt.Sprintf("failed to run diff: %v %v", out, err))

	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains("A /root/bar", line) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("couldn't find the new file in docker diff's output: %v", out)
	}
	deleteContainer(cleanCID)

	logDone("diff - check if created file shows up")
}

// test to ensure GH #3840 doesn't occur any more
func TestDiffEnsureDockerinitFilesAreIgnored(t *testing.T) {
	// this is a list of files which shouldn't show up in `docker diff`
	dockerinitFiles := []string{"/etc/resolv.conf", "/etc/hostname", "/etc/hosts", "/.dockerinit", "/.dockerenv"}

	// we might not run into this problem from the first run, so start a few containers
	for i := 0; i < 20; i++ {
		containerCmd := `echo foo > /root/bar`
		runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", containerCmd)
		cid, _, err := runCommandWithOutput(runCmd)
		errorOut(err, t, fmt.Sprintf("%s", err))

		cleanCID := stripTrailingCharacters(cid)

		diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
		out, _, err := runCommandWithOutput(diffCmd)
		errorOut(err, t, fmt.Sprintf("failed to run diff: %v %v", out, err))

		deleteContainer(cleanCID)

		for _, filename := range dockerinitFiles {
			if strings.Contains(out, filename) {
				t.Errorf("found file which should've been ignored %v in diff output", filename)
			}
		}
	}

	logDone("diff - check if ignored files show up in diff")
}

func TestDiffEnsureOnlyKmsgAndPtmx(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep 0")
	cid, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("%s", err))
	cleanCID := stripTrailingCharacters(cid)

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err := runCommandWithOutput(diffCmd)
	errorOut(err, t, fmt.Sprintf("failed to run diff: %v %v", out, err))
	deleteContainer(cleanCID)

	expected := map[string]bool{
		"C /dev":      true,
		"A /dev/full": true, // busybox
		"C /dev/ptmx": true, // libcontainer
		"A /dev/kmsg": true, // lxc
	}

	for _, line := range strings.Split(out, "\n") {
		if line != "" && !expected[line] {
			t.Errorf("'%s' is shown in the diff but shouldn't", line)
		}
	}
}
