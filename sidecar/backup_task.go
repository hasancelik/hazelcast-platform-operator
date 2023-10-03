package sidecar

import (
	"context"
	"path"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/hazelcast/platform-operator-agent/internal/bucket"
	"github.com/hazelcast/platform-operator-agent/internal/logger"
	"github.com/hazelcast/platform-operator-agent/internal/uri"
)

var backupLog = logger.New().Named("backup")

// task is an upload process that is cancelable
type task struct {
	req       UploadReq
	ctx       context.Context
	cancel    context.CancelFunc
	backupKey string
	bucketURI string
	key       string
	err       error
}

func (t *task) process(ID uuid.UUID) {
	backupLog.Info("task is started", zap.Uint32("task id", ID.ID()))

	defer backupLog.Info("task is finished", zap.Uint32("task id", ID.ID()))
	defer t.cancel()

	bucketURI, err := uri.NormalizeURI(t.req.BucketURL)
	if err != nil {
		backupLog.Error("error occurred while parsing bucket URI: "+err.Error(), zap.Uint32("task id", ID.ID()))
		t.err = err
		return
	}

	backupLog.Info("bucket URI successfully normalized", zap.String("bucket URI", bucketURI))

	secretData, err := bucket.SecretData(t.ctx, t.req.SecretName)
	if err != nil {
		backupLog.Error("error occurred while fetching secret: "+err.Error(), zap.Uint32("task ID", ID.ID()))
		t.err = err
		return
	}

	backupLog.Info("task successfully read secret", zap.Uint32("task id", ID.ID()), zap.String("secret name", t.req.SecretName))

	b, err := bucket.OpenBucket(t.ctx, bucketURI, secretData)
	if err != nil {
		backupLog.Error("task could not open the bucket: "+err.Error(), zap.Uint32("task id", ID.ID()))
		t.err = err
		return
	}

	backupsDir := path.Join(t.req.BackupBaseDir, DirName)

	backupLog.Info("Staring backup upload", zap.Uint32("task id", ID.ID()), zap.String("backupsDir", backupsDir), zap.Int("memberID", t.req.MemberID))
	folderKey, err := UploadBackup(t.ctx, b, backupsDir, t.req.HazelcastCRName, t.req.MemberID)
	if err != nil {
		backupLog.Error("task could not upload to the bucket: "+err.Error(), zap.Uint32("task id", ID.ID()))
		t.err = err
		return
	}

	backupLog.Info("task finished upload", zap.Uint32("task id", ID.ID()))

	backupKey, err := uri.Join(bucketURI, folderKey)
	if err != nil {
		backupLog.Error("task could format the URI: "+err.Error(), zap.Uint32("task id", ID.ID()))
		t.err = err
		return
	}

	t.err = b.Close()
	t.backupKey = backupKey
	t.bucketURI = bucketURI
	t.key = folderKey
}

func (t *task) cleanup(ctx context.Context) error {
	secretData, err := bucket.SecretData(ctx, t.req.SecretName)
	if err != nil {
		return err
	}

	return bucket.RemoveFile(ctx, t.bucketURI, t.key, secretData)
}
