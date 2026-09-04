package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the Kompa version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if jsonFlag {
				out := map[string]string{
					"version": Version,
					"os":      runtime.GOOS,
					"arch":    runtime.GOARCH,
					"go":      runtime.Version(),
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				fmt.Println(string(data))
				return
			}
			fmt.Printf("kompa %s (%s/%s, %s)\n", Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	return cmd
}
