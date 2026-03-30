package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	dot := flag.Bool("dot", false, "emit Graphviz DOT for neato -n instead of text")
	xdot := flag.Bool("xdot", false, "launch xdot viewer (requires neato and python3-xdot)")
	out := flag.String("o", "", "write output to file instead of stdout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: kbdash [flags] <path> [path ...]\n\n")
		fmt.Fprintf(os.Stderr, "Describe Kibana dashboards from integration packages.\n\n")
		fmt.Fprintf(os.Stderr, "Each path is a package directory (with kibana/dashboard/) or a\n")
		fmt.Fprintf(os.Stderr, "dashboard JSON file. Text output by default; use -dot for Graphviz.\n\n")
		fmt.Fprintf(os.Stderr, "  Render: kbdash -dot pkg/ | neato -n -Tsvg -o wireframe.svg\n")
		fmt.Fprintf(os.Stderr, "  View:   kbdash -xdot pkg/\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	files, err := findDashboardFiles(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no dashboard JSON files found")
		os.Exit(1)
	}

	var dashboards []DashboardInfo
	for _, f := range files {
		d, err := loadDashboard(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		dashboards = append(dashboards, extractDashboard(d, f))
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	switch {
	case *xdot:
		if err := launchXdot(dashboards); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case *dot:
		writeDOT(w, dashboards)
	default:
		describeText(w, dashboards)
	}
}

func launchXdot(dashboards []DashboardInfo) error {
	var buf bytes.Buffer
	writeDOT(&buf, dashboards)

	neato := exec.Command("neato", "-n", "-Txdot")
	neato.Stdin = &buf
	neato.Stderr = os.Stderr
	xdotData, err := neato.Output()
	if err != nil {
		return fmt.Errorf("neato: %w", err)
	}

	tmp, err := os.CreateTemp("", "kbdash-*.xdot")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(xdotData); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// xdot's -n flag has a bug: it sets filter=None, then crashes
	// trying to call [None, '-V'] for version detection. Work around
	// by monkey-patching before launching.
	pyScript := `
import sys, subprocess, re, signal
import xdot.ui.window as w
_orig = w.DotWidget.set_xdotcode
def _fix(self, xdotcode, center=True):
    if self.filter is None and self.graphviz_version is None:
        out = subprocess.check_output(["dot", "-V"], stderr=subprocess.STDOUT)
        m = re.match(rb".* version (?P<v>\S+)", out.rstrip())
        if m:
            self.graphviz_version = m.group("v").decode()
    return _orig(self, xdotcode, center=center)
w.DotWidget.set_xdotcode = _fix
sys.argv = ["xdot", "-n", sys.argv[1]]
from xdot.__main__ import main
main()
`
	cmd := exec.Command("python3", "-c", pyScript, tmp.Name())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
