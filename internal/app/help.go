package app

import (
	"fmt"
	"io"
)

func printHelp(w io.Writer, version string) {
	fmt.Fprintf(w, `informa-wizard — Informa Wizard (%s)

USAGE
  informa-wizard                     Launch interactive TUI
  informa-wizard <command> [flags]

COMMANDS
  install      Configure AI coding agents on this machine
  sync         Sync agent configs and skills to current version
  status       Print current installation state (plain text)
  doctor       Run diagnostic health checks on the current setup
  update       Check for available updates
  upgrade      Apply updates to managed tools
  restore      Restore a config backup
  version      Print version

FLAGS
  --help, -h    Show this help

Run 'informa-wizard help' for this message.
Documentation: https://github.com/Gentleman-Programming/informa-wizard
`, version)
}
