package cmd

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var getConfigCmd = &cobra.Command{
	Use:   "get-config",
	Short: "Gets the configuration for rpksi.",
	Long: `Gets the configuration for rpksi. The result is based on reading the config file and any flags being used.
Note: Flags will override any parameter values found in the config file.

Get config based on what is found in the config file and possibly default values:
	> rpksi get-config

Get config, but override the kafka value with a flag:
	> rpksi get-config --kafka redpanda-0:9092

`,
	Run: func(cmd *cobra.Command, args []string) {
		t := table.NewWriter()
		t.SetStyle(table.StyleLight)
		t.Style().Options.SeparateRows = true
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Parameter", "Value"})
		t.SortBy([]table.SortBy{
			{Name: "Parameter", Mode: table.Asc},
		})
		for k, v := range viper.AllSettings() {
			t.AppendRow(table.Row{k, v})
		}
		t.Render()
	},
}

func init() {
	rootCmd.AddCommand(getConfigCmd)
}
