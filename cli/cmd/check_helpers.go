package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type checkStatus string

const (
	checkStatusOK      checkStatus = "OK"
	checkStatusFailed  checkStatus = "FAILED"
	checkStatusSkipped checkStatus = "SKIPPED"
)

func reportCheck(cmd *cobra.Command, label string, err error) bool {
	status := checkStatusOK
	if err != nil {
		status = checkStatusFailed
	}
	return reportCheckStatus(cmd, label, status, err)
}

func reportCheckStatus(cmd *cobra.Command, label string, status checkStatus, err error) bool {
	switch status {
	case checkStatusFailed:
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "[FAILED] %s: %v\n", label, err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "[FAILED] %s\n", label)
		}
		return false
	case checkStatusSkipped:
		fmt.Fprintf(cmd.OutOrStdout(), "[SKIPPED] %s\n", label)
		return true
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "[OK] %s\n", label)
		return true
	}
}
