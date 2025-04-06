package libstandard

import (
	"fmt"
	"os"
	"reflect"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
)

// DefaultInitializer loads the config and initializes the logging.
// This assumes that there are "config" and "verbosity" flags present on the cobra-command.
func DefaultInitializer(cfg interface{}, cmd *cobra.Command, name string) error {
	config, err := cmd.Flags().GetString(Config)
	if err != nil {
		return err
	}

	err = Read(cfg, cmd.Flags(), config, DefaultFileConfig{Name: name, Extensions: []string{"yaml"}, Paths: []string{".", "~/.config/" + name}})
	if err != nil {
		return fmt.Errorf("an error occurred while reading the config! %w", err)
	}

	x := reflect.ValueOf(cfg).Elem()
	verbosity := x.FieldByName(strcase.ToCamel(Verbosity)).String()
	return SetupLogging(os.Stdout, verbosity)
}
