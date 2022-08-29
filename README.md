# rpksi - Redpanda Keeper Shadow Indexing CLI

`rpksi` provides visibility to and management of topic segments found in your S3-compatible storage layer.

## Getting started

Clone repo:
```shell
git clone https://github.com/vuldin/rpksi-release.git
cd rpksi-release
```

Install go dependencies:

```shell
go get github.com/minio/minio-go/v7
go get -u github.com/spf13/cobra@latest
go get github.com/spf13/viper
go get github.com/jedib0t/go-pretty/v6/table
```

Print the help menu:

```shell
go run main.go
```

Add a config file:

```shell
cat <<EOF > rpksi.yaml
kafka: "localhost:9092"
s3: "localhost:9000"
admin: "localhost:9644"
region: "local"
bucket: "redpanda"
accessKey: "minio"
secretKey: "minio123"
useSSL: false
EOF
```

The help menu shows details on sub-commands, flags, and other details. Each sub-command has its own help menu with further details.

# rpksi use case

Imagine a scenario where a user wants to remove some objects being stored to save money. A topic with shadow indexing enabled is storing segments in a bucket that is getting large.

First get a list of all topics:

```shell
> go run main.go ls           
┌────────┬───────────┬───────────────┬─────────────┬───────────────┐
│ TOPIC  │ SIZE      │ SEGMENT COUNT │ BASE OFFSET │ NEWEST OFFSET │
├────────┼───────────┼───────────────┼─────────────┼───────────────┤
│ atopic │ 90.80 KiB │             8 │           0 │         11007 │
└────────┴───────────┴───────────────┴─────────────┴───────────────┘
```

We can see a single topic with a size of 90.80 KiB across 8 segments with an offset range of 0 to 11007. Now view details on each of the topic's segments:

```shell
> go run main.go ls -a        
┌────────────────────┬────────────────────────────────────────────────────────────────────────────────────────────┐
│        TOPIC       │                                           SEGMENT                                          │
├────────┬───────────┼────────────────┬───────────┬───────────────┬───────────────┬───────────────┬───────────────┤
│ NAME   │ SIZE      │      NAME      │    SIZE   │         OLDEST OFFSET         │         NEWEST OFFSET         │
│        │           ├────────────────┼───────────┼───────────────┬───────────────┼───────────────┬───────────────┤
│        │           │                │           │       #       │      DATE     │       #       │      DATE     │
├────────┼───────────┼────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│ atopic │ 90.80 KiB │   0-1-v1.log   │  8.28 KiB │       0       │ 1653706984642 │      1000     │ 1653707348951 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  1001-1-v1.log │  8.26 KiB │      1001     │ 1653707358740 │      2001     │ 1653707359073 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  2002-1-v1.log │ 16.19 KiB │      2002     │ 1653707369361 │      4001     │ 1653707377841 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  4002-1-v1.log │  8.43 KiB │      4002     │ 1653707378815 │      5003     │ 1653707439180 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  5004-1-v1.log │  8.67 KiB │      5004     │ 1653707449074 │      6004     │ 1653707449510 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  6005-1-v1.log │ 16.19 KiB │      6005     │ 1653707452959 │      8004     │ 1653707456317 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  8005-1-v1.log │ 16.52 KiB │      8005     │ 1653707459153 │     10006     │ 1653707468798 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │ 10007-1-v1.log │  8.26 KiB │     10007     │ 1653707472972 │     11007     │ 1653707479296 │
└────────┴───────────┴────────────────┴───────────┴───────────────┴───────────────┴───────────────┴───────────────┘
```

The tool allows you to filter segments to show those associated with a topic, or to show those segments which are older than (and do not contain) an offset and/or timestamp.

You want to check the data at offset 4500 to see if it's worth keeping:

```shell
> rpk topic consume atopic -o 4500 -n 1
{
  "topic": "atopic",
  "value": "Fri May 27 11:10:39 PM EDT 2022 501",
  "timestamp": 1653707439180,
  "partition": 0,
  "offset": 4500
}
```

The data looks to be the oldest valuable data you want to keep, so now find those segments that contain offsets which are older than offset 4500:

```shell
> go run main.go ls -ao 4500                           
┌────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────┐
│        TOPIC       │                                          SEGMENT                                          │
├────────┬───────────┼───────────────┬───────────┬───────────────┬───────────────┬───────────────┬───────────────┤
│ NAME   │ SIZE      │      NAME     │    SIZE   │         OLDEST OFFSET         │         NEWEST OFFSET         │
│        │           ├───────────────┼───────────┼───────────────┬───────────────┼───────────────┬───────────────┤
│        │           │               │           │       #       │      DATE     │       #       │      DATE     │
├────────┼───────────┼───────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│ atopic │ 32.73 KiB │   0-1-v1.log  │  8.28 KiB │       0       │ 1653706984642 │      1000     │ 1653707348951 │
│        │           ├───────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │ 1001-1-v1.log │  8.26 KiB │      1001     │ 1653707358740 │      2001     │ 1653707359073 │
│        │           ├───────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │ 2002-1-v1.log │ 16.19 KiB │      2002     │ 1653707369361 │      4001     │ 1653707377841 │
└────────┴───────────┴───────────────┴───────────┴───────────────┴───────────────┴───────────────┴───────────────┘
```

There are three segments that contain older offsets and can be deleted. Do a dry run of the deletion:

```shell
> go run main.go del -o 4500 --dry-run
Dry run (no changes being made)...
Generating new manifest...
  removing segment 0-1-v1.log
  removing segment 1001-1-v1.log
  removing segment 2002-1-v1.log
Generating new manifest complete
{"last_offset":11007,"namespace":"kafka","partition":0,"revision":3,"segments":{"10007-1-v1.log":{"is_compacted":false,"size_bytes":8460,"committed_offset":11007,"base_offset":10007,"base_timestamp":1653707472972,"max_timestamp":1653707479296,"delta_offset":7,"archiver_term":1},"4002-1-v1.log":{"is_compacted":false,"size_bytes":8633,"committed_offset":5003,"base_offset":4002,"base_timestamp":1653707378815,"max_timestamp":1653707439180,"delta_offset":2,"archiver_term":1},"5004-1-v1.log":{"is_compacted":false,"size_bytes":8882,"committed_offset":6004,"base_offset":5004,"base_timestamp":1653707449074,"max_timestamp":1653707449510,"delta_offset":4,"archiver_term":1},"6005-1-v1.log":{"is_compacted":false,"size_bytes":16574,"committed_offset":8004,"base_offset":6005,"base_timestamp":1653707452959,"max_timestamp":1653707456317,"delta_offset":5,"archiver_term":1},"8005-1-v1.log":{"is_compacted":false,"size_bytes":16920,"committed_offset":10006,"base_offset":8005,"base_timestamp":1653707459153,"max_timestamp":1653707468798,"delta_offset":5,"archiver_term":1}},"topic":"atopic","version":1}
Deleting segments...
   491bc459/kafka/atopic/0_3/0-1-v1.log.1
   fb104934/kafka/atopic/0_3/1001-1-v1.log.1
   31e19f9e/kafka/atopic/0_3/2002-1-v1.log.1
Dry run complete
```

We see the same three segments identified with the list command, a newly generated manifest that excludes those segments, and the paths to the related objects in S3. The details look correct, so now we remove the `dry-run` flag from the delete command:

```shell
> go run main.go del -o 4500          
Generating new manifest...
  removing segment 1001-1-v1.log
  removing segment 2002-1-v1.log
  removing segment 0-1-v1.log
Generating new manifest complete
Deleting segments...
   491bc459/kafka/atopic/0_3/0-1-v1.log.1
   fb104934/kafka/atopic/0_3/1001-1-v1.log.1
   31e19f9e/kafka/atopic/0_3/2002-1-v1.log.1
```

Once the new manifest is generated, then it is pushed to S3 (overwriting the previous manifest), and then the unneeded segments are deleted. Verify with the list command (without the previous filters):

```shell
> go run main.go ls -a                
┌────────────────────┬────────────────────────────────────────────────────────────────────────────────────────────┐
│        TOPIC       │                                           SEGMENT                                          │
├────────┬───────────┼────────────────┬───────────┬───────────────┬───────────────┬───────────────┬───────────────┤
│ NAME   │ SIZE      │      NAME      │    SIZE   │         OLDEST OFFSET         │         NEWEST OFFSET         │
│        │           ├────────────────┼───────────┼───────────────┬───────────────┼───────────────┬───────────────┤
│        │           │                │           │       #       │      DATE     │       #       │      DATE     │
├────────┼───────────┼────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│ atopic │ 58.08 KiB │  4002-1-v1.log │  8.43 KiB │      4002     │ 1653707378815 │      5003     │ 1653707439180 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  5004-1-v1.log │  8.67 KiB │      5004     │ 1653707449074 │      6004     │ 1653707449510 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  6005-1-v1.log │ 16.19 KiB │      6005     │ 1653707452959 │      8004     │ 1653707456317 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │  8005-1-v1.log │ 16.52 KiB │      8005     │ 1653707459153 │     10006     │ 1653707468798 │
│        │           ├────────────────┼───────────┼───────────────┼───────────────┼───────────────┼───────────────┤
│        │           │ 10007-1-v1.log │  8.26 KiB │     10007     │ 1653707472972 │     11007     │ 1653707479296 │
└────────┴───────────┴────────────────┴───────────┴───────────────┴───────────────┴───────────────┴───────────────┘
```

