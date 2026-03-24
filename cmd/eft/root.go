package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "eft",
	Short: "Create ANSI/NIST-ITL (EFT/EBTS) biometric transaction files",
	Long: `eft creates ANSI/NIST-ITL electronic fingerprint transmission files
for biometric submissions. Supports generic transactions, ATF eForms
(Form 1/Form 4), and FD-258 card processing.`,
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(atfCmd)
	rootCmd.AddCommand(atfImagesCmd)
	rootCmd.AddCommand(cropCmd)
}
