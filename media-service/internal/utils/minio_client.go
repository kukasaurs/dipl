package utils

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func NewMinioClient(endpoint, accessKey, secretKey string, bucketName string) (*minio.Client, error) {
	// Создаём клиента
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // false — если без HTTPS
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Проверка существования bucket
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	// Если нет — создаём и ставим политику
	if !exists {
		err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}

		// Публичная read-only политика
		publicPolicy := `{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Action": ["s3:GetObject"],
					"Effect": "Allow",
					"Principal": "*",
					"Resource": "arn:aws:s3:::` + bucketName + `/*"
				}
			]
		}`

		err = client.SetBucketPolicy(ctx, bucketName, publicPolicy)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}
