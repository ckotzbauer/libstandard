package root

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestSetupLogging(t *testing.T) {
	err := SetupLogging(os.Stdout, "unknown")
	assert.NotNil(t, err)
	assert.NotEqual(t, "unknown", logrus.GetLevel().String())

	err = SetupLogging(os.Stdout, "info")
	assert.Nil(t, err)
	assert.Equal(t, os.Stdout, logrus.StandardLogger().Out)
	assert.Equal(t, "info", logrus.GetLevel().String())
}

func TestAddVerbosityFlag(t *testing.T) {
	cmd := &cobra.Command{}
	AddVerbosityFlag(cmd)
	assert.NotNil(t, cmd.PersistentFlags().Lookup(Verbosity))
}
