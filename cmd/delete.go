package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type void struct{}

var member void
var adminClient = &http.Client{Timeout: 10 * time.Second}
var deleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"del"},
	Short:   "Deletes one or more segments from S3 starting with the oldest.",
	Long: `Deletes one or more segments from S3 starting with the oldest.

THIS TOOL SHOULD BE USED WITH CAUTION. This tool deletes objects from your S3
storage representing topic segments, which may be the only copy of this information.
Make sure to enter the correct topic and timestamp that identifies only those segments
you wish to delete. The delete command requires both a topic and timestamp flag to function.

You can find the appropriate topic and timestamp values with the list subcommand. See below
for an overview, and also see 'rpksi help list' for more details

Without any arguments, delete prints this help menu:
	> rpksi delete

List segments for all topics sorted by size:
	> rpksi list -a

Narrow down results to a specific topic:
	> rpksi list -at aTopic

View the event at offset 4500 for a topic:
	> rpk topic consume -n 1 aTopic -o 4500

Do a dry-run that deletes segments containing only those offsets that are older than 4500:
	> rpksi del -o 4500 --dry-run

Delete the segments found above:
	> rpksi del -o 4500
`,
	Run: func(cmd *cobra.Command, args []string) {
		topicFlag, _ := cmd.Flags().GetString("topic")
		if len(topicFlag) == 0 {
			log.Fatalln("Topic required (--topic or -t)")
		}
		dryrunFlag, _ := cmd.Flags().GetBool("dry-run")
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
		adminApi := viper.GetString("admin")
		s3Api := viper.GetString("s3")
		bucket := viper.GetString("bucket")
		accessKeyID := viper.GetString("accessKey")
		secretAccessKey := viper.GetString("secretKey")
		useSSL := viper.GetBool("useSSL")

		isDeleting := false
		namespace := "kafka"
		manifestFileName := "manifest.json"
		protocol := "http"
		if useSSL {
			protocol = "https"
		}

		manifests := make(map[string]Manifest)
		segments := make(map[string]RowSegment)
		manifestEntries := make(map[string]RowSegment)
		urls := make(map[string]void)

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
				fmt.Println(object.Err)
				return
			}
			if strings.Split(object.Key, "/")[2] != namespace {
				topic := strings.Split(object.Key, "/")[2]
				// collect remote segment paths
				objectKey := strings.Split(object.Key, "/")[2] + ":" + strings.Split(object.Key, "/")[4]
				objectKey = strings.TrimRight(objectKey, ".1")
				segments[objectKey] = RowSegment{
					ObjectPath: object.Key,
					TopicName:  topic,
				}
			} else {
				if strings.Contains(object.Key, "/"+manifestFileName) {
					// read remote manifest and populate manifestEntries map
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

					err = json.Unmarshal(data, &manifest)
					if err != nil {
						log.Fatalln(err)
					}

					// continue only if this manifest relates to the given topic
					if len(topicFlag) > 0 && topicFlag != manifest.Topic {
						continue
					}

					// add manifest to manifests
					manifests[object.Key] = manifest

					// filter manifest entries based on given filters, then add to manifestEntries map
					for key, val := range manifest.Segments {
						manifestEntries[manifest.Topic+":"+key] = RowSegment{
							Partition:            manifest.Partition,
							SegmentName:          key,
							SegmentSize:          byteCountBinary(val.SizeBytes),
							SegmentOldOffsetDate: val.BaseTimestamp,
							SegmentNewOffsetDate: val.MaxTimestamp,
							SegmentOldOffsetId:   val.BaseOffset,
							SegmentNewOffsetId:   val.CommittedOffset,
							TopicName:            manifest.Topic,
						}
					}
				}
			}
		}

		// expand segments data based on manifestEntries
		for sk, sv := range segments {
			for mek, mev := range manifestEntries {
				if sk == mek {
					mev.ObjectPath = sv.ObjectPath
					mev.Delete = false

					// filter segments
					if len(olderThanFlag) > 0 && olderThanTimestamp > int64(mev.SegmentNewOffsetDate) {
						mev.Delete = true
						isDeleting = true
					}
					if offsetFlag != -1 && offsetFlag > int64(mev.SegmentNewOffsetId) {
						mev.Delete = true
						isDeleting = true
					}

					segments[sk] = mev
				}
			}
		}

		if isDeleting {
			if dryrunFlag {
				fmt.Println("Dry run (no changes being made)...")
			}

			fmt.Println("Deleting segments...")
			for _, v := range segments {
				if v.Delete {
					fmt.Println("  delete " + v.ObjectPath)
					if !dryrunFlag {
						err = s3Client.RemoveObject(context.Background(), bucket, v.ObjectPath, minio.RemoveObjectOptions{GovernanceBypass: true})
						if err != nil {
							log.Fatalln(err)
						}
					}
				}
			}

			fmt.Println("Synchronizing local state...")
			for _, v := range segments {
				url := fmt.Sprintf("%s://%s/v1/shadow_indexing/sync_local_state/%s/%d", protocol, adminApi, v.TopicName, v.Partition)
				urls[url] = member
			}
			for url := range urls {
				if dryrunFlag {
					fmt.Println("  POST request to " + url)
				} else {
					request, err := http.NewRequest("POST", url, new(bytes.Buffer))
					response, err := adminClient.Do(request)
					if err != nil {
						panic(err)
					}
					defer response.Body.Close()
				}
			}

			fmt.Println("Determining if manifest segments should be removed...")
			for mk, mv := range manifests {
				for msk, msv := range mv.Segments {
					for _, sv := range segments {
						if sv.Delete && sv.SegmentOldOffsetId == msv.BaseOffset && sv.SegmentNewOffsetId == msv.CommittedOffset {
							// found a manifest segment that needs to be deleted
							mv.NeedsRewrite = true
							fmt.Println("  removing segment", msk)
							delete(mv.Segments, msk)
						}
					}
				}
				manifests[mk] = mv
			}

			fmt.Println("Writing new manifest...")
			for k, manifest := range manifests {
				if manifest.NeedsRewrite {
					json, err := json.Marshal(manifest)
					if err != nil {
						log.Fatalln(err)
					}
					if dryrunFlag {
						fmt.Println(string(json))
					} else {
						reader := strings.NewReader(string(json))
						uploadInfo, err := s3Client.PutObject(context.Background(), bucket, k, reader, -1, minio.PutObjectOptions{})
						if err != nil {
							log.Fatalln(err)
							return
						}
						fmt.Println("  uploaded manifest " + uploadInfo.Key)
					}
				}
			}

			if dryrunFlag {
				fmt.Println("Dry run complete")
			} else {
				fmt.Println("Complete.")
			}
		} else {
			fmt.Println("no segments found (try another topic, increasing the offset, or a more recent timestamp")
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolP("all", "a", false, "ignored on delete (included for switching between list and delete easily)")
	deleteCmd.Flags().StringP("topic", "t", "", "filter by topic")
	deleteCmd.Flags().StringP("older-than", "", "", "show segments w/ offsets older than timestamp (exclusive)")
	deleteCmd.Flags().Int64P("offset", "o", -1, "show segments containing an offset range that is lower than the given offset")
	deleteCmd.Flags().Bool("dry-run", false, "dry run, prints each task output to console")
}
