package main

import "io"

type DownloadState struct {
	TotalBucketSize             int64 `json:"totalBucketSize"`
	DownloadedBucketSize        int64 `json:"downloadedBucketSize"`
	TotalObjectCount            int64   `json:"totalObjectCount"`
	DownloadedObjectCount       int64   `json:"downloadedObjectCount"`
	CurrentObjectSize           int64 `json:"currentObjectSize"`
	CurrentDownloadedObjectSize int64 `json:"currentDownloadedObjectSize"`
	State                       int64   `json:"state"`
}

type progressWriter struct {
	writer        io.WriterAt
	downloadState *DownloadState
}
