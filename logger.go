libstandard

import (
	"io"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//SetupLogging set the log output as the log level
func SetupLogging(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	logrus.SetLevel(lvl)
	return nil
}

func AddVerbosityFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(Verbosity, "v", "", "Log-level (debug, info, warn, error, fatal, panic)")
}
