package serve

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ReadFlagsFromFile checks for '@' prefix, if found try to read value from file.
func ReadFlagsFromFile(cmd *cobra.Command, names ...string) error {
	for _, name := range names {
		value, err := cmd.Flags().GetString(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to fetch value for key %s\n", name)
			os.Exit(1)
		}
		if strings.HasPrefix(value, "@") {

			value = value[1:]
			data, err := os.ReadFile(value)
			if err != nil {
				return fmt.Errorf(
					"can't read value of flag '%s' from file '%s': %w",
					name, value, err,
				)
			}
			value = strings.TrimSpace(string(data))

			err = cmd.Flags().Set(name, value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unable to set value for key %s\n", name)
				os.Exit(1)
			}
		}
	}
	return nil
}
