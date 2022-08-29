package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var cfgFile string

type Segment struct {
	IsCompacted     bool   `json:"is_compacted"`
	SizeBytes       uint64 `json:"size_bytes"`
	CommittedOffset uint64 `json:"committed_offset"`
	BaseOffset      uint64 `json:"base_offset"`
	BaseTimestamp   uint64 `json:"base_timestamp"`
	MaxTimestamp    uint64 `json:"max_timestamp"`
	DeltaOffset     uint64 `json:"delta_offset"`
	ArchiverTerm    int    `json:"archiver_term"`
}

type Manifest struct {
	Version      int                `json:"version"`
	Namespace    string             `json:"namespace"`
	Topic        string             `json:"topic"`
	Partition    int                `json:"partition"`
	Revision     int                `json:"revision"`
	LastOffset   uint64             `json:"last_offset"`
	Segments     map[string]Segment `json:"segments"`
	NeedsRewrite bool
}

type RowTopic struct {
	TopicName          string
	TopicSize          string
	SegmentCount       int
	SegmentOldOffsetId uint64
	SegmentNewOffsetId uint64
}

type RowSegment struct {
	Delete               bool
	ObjectPath           string
	Partition            int
	TopicName            string
	SegmentName          string
	SegmentSize          string
	SegmentOldOffsetDate uint64
	SegmentNewOffsetDate uint64
	SegmentOldOffsetId   uint64
	SegmentNewOffsetId   uint64
}

// MarshalJSON excludes NeedsRewrite from json manifest
func (m Manifest) MarshalJSON() ([]byte, error) {
	mMap := map[string]interface{}{
		"version":     m.Version,
		"namespace":   m.Namespace,
		"topic":       m.Topic,
		"partition":   m.Partition,
		"revision":    m.Revision,
		"last_offset": m.LastOffset,
		"segments":    m.Segments,
	}
	return json.Marshal(mMap)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rpksi",
	Short: "Redpanda Keeper Shadow Indexing management tool",
	Long: `rpksi provides visibility to and management of topic segments found in your S3-compatible storage layer.

Running rpksi without arguments prints this help menu.

Values in a config file will be used if found (default location is $HOME/.redpanda/rpksi.yaml). An example config file:

	kafka: "localhost:9092"
	admin: "localhost:9644"
	s3: "localhost:9000"
	region: "local"
	bucket: "redpanda"
	accessKey: "minio"
	secretKey: "minio123"
	useSSL: false

USAGE:

	Print this help menu:
		rpksi [ help | --help | -h ]

	Print the help menu for sub-commands:
		rpksi [ help ] ls [--help | -h ]
`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			return
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./rpksi.yaml)")
	rootCmd.PersistentFlags().StringP("kafka", "", "localhost:9092", "Kafka endpoint")
	rootCmd.PersistentFlags().StringP("admin", "", "localhost:9644", "Admin endpoint")
	rootCmd.PersistentFlags().StringP("s3", "", "localhost:9000", "S3 endpoint")
	rootCmd.PersistentFlags().StringP("bucket", "b", "redpanda", "bucket name")
	rootCmd.PersistentFlags().String("accessKey", "", "access key")
	rootCmd.PersistentFlags().String("secretKey", "", "secret key")

	cobra.OnInitialize(initConfig)

	/*
		// TODO not working correctly
		rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
			if !f.Changed && viper.IsSet(f.Name) {
				val := viper.Get(f.Name)
				rootCmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
			}
		})
	*/
}

// initConfig reads in config from file or environment variables if set.
func initConfig() {
	if cfgFile != "" {
		fmt.Println("using flag to find config file")
		fmt.Println(cfgFile)
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory
		cwd, err := os.Getwd()
		cobra.CheckErr(err)
		viper.AddConfigPath(cwd)
		viper.SetConfigType("yaml")
		viper.SetConfigName("rpksi")

		// If a config file is not found, read it in. Return error on parsing issue.
		if err := viper.ReadInConfig(); err != nil {
			// It's okay if there isn't a config file
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				/*
					fmt.Println("Unable to parse config file.")
					os.Exit(1)
				*/
				return
			}
		}
	}

	// Load flags values into viper config (will be default value if not specified).
	viper.BindPFlag("kafka", rootCmd.Flags().Lookup("kafka"))
	viper.BindPFlag("admin", rootCmd.Flags().Lookup("admin"))
	viper.BindPFlag("s3", rootCmd.Flags().Lookup("s3"))
	viper.BindPFlag("bucket", rootCmd.Flags().Lookup("bucket"))
	viper.BindPFlag("accessKey", rootCmd.Flags().Lookup("accessKey"))
	viper.BindPFlag("secretKey", rootCmd.Flags().Lookup("secretKey"))

	viper.AutomaticEnv()
}
