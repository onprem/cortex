package bucketindex

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanos-io/objstore"

	"github.com/cortexproject/cortex/pkg/storage/bucket"
	cortex_testutil "github.com/cortexproject/cortex/pkg/storage/tsdb/testutil"
)

func TestGlobalMarkersBucket_Delete_ShouldSucceedIfDeletionMarkDoesNotExistInTheBlockButExistInTheGlobalLocation(t *testing.T) {
	bkt, _ := cortex_testutil.PrepareFilesystemBucket(t)

	ctx := context.Background()
	bkt = BucketWithGlobalMarkers(bkt)

	// Create a mocked block deletion mark in the global location.
	blockID := ulid.MustNew(1, nil)
	globalPath := BlockDeletionMarkFilepath(blockID)
	require.NoError(t, bkt.Upload(ctx, globalPath, strings.NewReader("{}")))

	// Ensure it exists before deleting it.
	ok, err := bkt.Exists(ctx, globalPath)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, bkt.Delete(ctx, globalPath))

	// Ensure has been actually deleted.
	ok, err = bkt.Exists(ctx, globalPath)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestGlobalMarkersBucket_isBlockDeletionMark(t *testing.T) {
	block1 := ulid.MustNew(1, nil)

	tests := []struct {
		name       string
		expectedOk bool
		expectedID ulid.ULID
	}{
		{
			name:       "",
			expectedOk: false,
		}, {
			name:       "deletion-mark.json",
			expectedOk: false,
		}, {
			name:       block1.String() + "/index",
			expectedOk: false,
		}, {
			name:       block1.String() + "/deletion-mark.json",
			expectedOk: true,
			expectedID: block1,
		}, {
			name:       "/path/to/" + block1.String() + "/deletion-mark.json",
			expectedOk: true,
			expectedID: block1,
		},
	}

	b := BucketWithGlobalMarkers(nil).(*globalMarkersBucket)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualOk := b.isBlockDeletionMark(tc.name)
			assert.Equal(t, tc.expectedOk, actualOk)
			assert.Equal(t, tc.expectedID, actualID)
		})
	}
}

func TestBucketWithGlobalMarkers_ShouldWorkCorrectlyWithBucketMetrics(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	ctx := context.Background()

	// We wrap the underlying filesystem bucket client with metrics,
	// global markers (intentionally in the middle of the chain) and
	// user prefix.
	bkt, _ := cortex_testutil.PrepareFilesystemBucket(t)
	bkt = objstore.BucketWithMetrics("", bkt, reg)
	bkt = BucketWithGlobalMarkers(bkt)
	userBkt := bucket.NewUserBucketClient("user-1", bkt, nil)

	reader, err := userBkt.Get(ctx, "does-not-exist")
	require.Error(t, err)
	require.Nil(t, reader)
	assert.True(t, bkt.IsObjNotFoundErr(err))

	// Should track the failure.
	assert.NoError(t, testutil.GatherAndCompare(reg, bytes.NewBufferString(`
		# HELP thanos_objstore_bucket_operation_failures_total Total number of operations against a bucket that failed, but were not expected to fail in certain way from caller perspective. Those errors have to be investigated.
		# TYPE thanos_objstore_bucket_operation_failures_total counter
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="attributes"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="delete"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="exists"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="get"} 1
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="get_range"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="iter"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="upload"} 0
		# HELP thanos_objstore_bucket_operations_total Total number of all attempted operations against a bucket.
		# TYPE thanos_objstore_bucket_operations_total counter
		thanos_objstore_bucket_operations_total{bucket="",operation="attributes"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="delete"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="exists"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="get"} 1
		thanos_objstore_bucket_operations_total{bucket="",operation="get_range"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="iter"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="upload"} 0
	`),
		"thanos_objstore_bucket_operations_total",
		"thanos_objstore_bucket_operation_failures_total",
	))

	reader, err = userBkt.ReaderWithExpectedErrs(userBkt.IsObjNotFoundErr).Get(ctx, "does-not-exist")
	require.Error(t, err)
	require.Nil(t, reader)
	assert.True(t, bkt.IsObjNotFoundErr(err))

	// Should not track the failure.
	assert.NoError(t, testutil.GatherAndCompare(reg, bytes.NewBufferString(`
		# HELP thanos_objstore_bucket_operation_failures_total Total number of operations against a bucket that failed, but were not expected to fail in certain way from caller perspective. Those errors have to be investigated.
		# TYPE thanos_objstore_bucket_operation_failures_total counter
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="attributes"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="delete"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="exists"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="get"} 1
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="get_range"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="iter"} 0
		thanos_objstore_bucket_operation_failures_total{bucket="",operation="upload"} 0
		# HELP thanos_objstore_bucket_operations_total Total number of all attempted operations against a bucket.
		# TYPE thanos_objstore_bucket_operations_total counter
		thanos_objstore_bucket_operations_total{bucket="",operation="attributes"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="delete"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="exists"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="get"} 2
		thanos_objstore_bucket_operations_total{bucket="",operation="get_range"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="iter"} 0
		thanos_objstore_bucket_operations_total{bucket="",operation="upload"} 0
	`),
		"thanos_objstore_bucket_operations_total",
		"thanos_objstore_bucket_operation_failures_total",
	))
}
