package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

func byteCountBinary(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Lists details for each topic managed by shadow indexing",
	Long: `Lists details for each topic managed by shadow indexing.

By default it prints a table showing each topic (sorted by size) and a few details:
	> rpksi list

For more details add --all:
	> rpksi list --all

Filter by topic with --topic:
	> rpksi list --topic aTopic

Find the storage size and segment count for aTopic containing offsets that are older than the given unix timestamp:
	> rpksi list --older-than 3546080250619836472

List segment details for segments with offsets older than the given unix timestamp, filtering by topic:
	> rpksi list -a --topic aTopic --older-than 3546080250619836472
`,
	Run: func(cmd *cobra.Command, args []string) {
		allFlag, _ := cmd.Flags().GetBool("all")
		topicFlag, _ := cmd.Flags().GetString("topic")
		olderThanFlag, _ := cmd.Flags().GetString("older-than")
		offsetFlag, _ := cmd.Flags().GetInt64("offset")
		var olderThanTimestamp int64
		if len(olderThanFlag) > 0 {
			var err error
			olderThanTimestamp, err = strconv.ParseInt(olderThanFlag, 10, 64)
			if err != nil {
				log.Fatalln(err)
			}
		}
		s3Api := viper.GetString("s3")
		bucket := viper.GetString("bucket")
		accessKeyID := viper.GetString("accessKey")
		secretAccessKey := viper.GetString("secretKey")
		useSSL := viper.GetBool("useSSL")

		//manifestHashPrefixRegexp := regexp.MustCompile("([a-z]|\\d){1,3}0{6,8}")
		manifestFileName := "manifest.json"
		namespace := "kafka"

		//objectPaths := make(map[string]string)
		segments := make(map[string]RowSegment)
		topics := make(map[string]RowTopic)

		t := table.NewWriter()
		t.SetStyle(table.StyleLight)
		t.Style().Options.SeparateRows = true
		t.SetOutputMirror(os.Stdout)
		rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
		if allFlag {
			t.SetColumnConfigs([]table.ColumnConfig{
				{Number: 1, AutoMerge: true},
				{Number: 2, AutoMerge: true},
				{Number: 3, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
				{Number: 4, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
				{Number: 5, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
				{Number: 6, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
				{Number: 7, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
				{Number: 8, Align: text.AlignCenter, AlignFooter: text.AlignCenter, AlignHeader: text.AlignCenter},
			})
			t.AppendHeader(table.Row{"Topic", "Topic", "Remote Segment", "Remote Segment", "Remote Segment", "Remote Segment", "Remote Segment", "Remote Segment"}, rowConfigAutoMerge)
			t.AppendHeader(table.Row{"Name", "Size", "Name", "Size", "Oldest Offset", "Oldest Offset", "Newest Offset", "Newest Offset"}, rowConfigAutoMerge)
			t.AppendHeader(table.Row{"Name", "Size", "", "", "#", "Date", "#", "Date"})
			t.SortBy([]table.SortBy{
				{Number: 1, Mode: table.Asc},
				{Number: 5, Mode: table.AscNumeric},
			})
		} else {
			t.AppendHeader(table.Row{"Topic", "Size", "Remote Segment Count", "Base Remote Offset", "Newest Remote Offset"})
			t.SortBy([]table.SortBy{
				{Name: "Topic", Mode: table.Asc},
			})
		}

		s3Client, err := minio.New(s3Api, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			fmt.Println(err)
			return
		}

		// loop through top-level objects (hash prefix names)
		for object := range s3Client.ListObjects(context.Background(), bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				panic(object.Err)
			}
			if strings.Split(object.Key, "/")[2] == namespace {
				// read manifest to list remote segments
				if strings.Contains(object.Key, "/"+manifestFileName) {
					reader, err := s3Client.GetObject(context.Background(), bucket, object.Key, minio.GetObjectOptions{})
					if err != nil {
						log.Fatalln(err)
					}
					defer func(reader *minio.Object) {
						err := reader.Close()
						if err != nil {
							log.Fatalln(err)
						}
					}(reader)

					data, _ := io.ReadAll(reader)

					var manifest Manifest
					var totalSizeBytes uint64
					var baseOffset uint64

					err = json.Unmarshal(data, &manifest)
					if err != nil {
						log.Fatalln(err)
					}

					// topic filter
					if len(topicFlag) > 0 && topicFlag != manifest.Topic {
						continue
					}

					var passingSegmentCount int
					for key, val := range manifest.Segments {
						//fmt.Println(key, val.BaseOffset, val.CommittedOffset, val.DeltaOffset)
						if len(olderThanFlag) > 0 && olderThanTimestamp <= int64(val.MaxTimestamp) {
							continue
						}
						if offsetFlag != -1 && offsetFlag <= int64(val.CommittedOffset) {
							continue
						}
						passingSegmentCount++

						// RowTopic values
						if val.BaseOffset < baseOffset {
							baseOffset = val.BaseOffset
						}
						totalSizeBytes += val.SizeBytes

						// RowSegment values
						// TODO find segmentObject.key
						segments[manifest.Topic+":"+key] = RowSegment{
							//ObjectName:           segmentObject.Key,
							TopicName:            manifest.Topic,
							SegmentName:          key,
							SegmentSize:          byteCountBinary(val.SizeBytes),
							SegmentOldOffsetDate: val.BaseTimestamp,
							SegmentNewOffsetDate: val.MaxTimestamp,
							SegmentOldOffsetId:   val.BaseOffset,
							SegmentNewOffsetId:   val.CommittedOffset,
						}
					}

					topics[manifest.Topic] = RowTopic{
						TopicName:          manifest.Topic,
						TopicSize:          byteCountBinary(totalSizeBytes),
						SegmentCount:       passingSegmentCount,
						SegmentOldOffsetId: baseOffset,
						SegmentNewOffsetId: manifest.LastOffset,
					}
				}
			}
		}

		if allFlag {
			for _, segment := range segments {
				topic := topics[segment.TopicName]
				t.AppendRow(table.Row{
					topic.TopicName,
					topic.TopicSize,
					segment.SegmentName,
					segment.SegmentSize,
					segment.SegmentOldOffsetId,
					segment.SegmentOldOffsetDate,
					segment.SegmentNewOffsetId,
					segment.SegmentNewOffsetDate,
				})
			}
		} else {
			for _, topic := range topics {
				t.AppendRow(table.Row{
					topic.TopicName,
					topic.TopicSize,
					topic.SegmentCount,
					topic.SegmentOldOffsetId,
					topic.SegmentNewOffsetId,
				})
			}
		}
		t.Render()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolP("all", "a", false, "show all details")
	listCmd.Flags().StringP("topic", "t", "", "filter by topic")
	listCmd.Flags().StringP("older-than", "", "", "show segments w/ offsets older than timestamp (exclusive)")
	listCmd.Flags().Int64P("offset", "o", -1, "show segments containing an offset range that is lower than the given offset")
}
