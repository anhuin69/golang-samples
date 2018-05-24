// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/golang-samples/internal/testutil"
)

func TestObjects(t *testing.T) {
	tc := testutil.SystemTest(t)
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var (
		bucket    = tc.ProjectID + "-samples-object-bucket-1"
		dstBucket = tc.ProjectID + "-samples-object-bucket-2"

		object1 = "foo.txt"
		object2 = "foo/a.txt"
	)

	cleanBucket(t, ctx, client, tc.ProjectID, bucket)
	cleanBucket(t, ctx, client, tc.ProjectID, dstBucket)

	if err := write(client, bucket, object1); err != nil {
		t.Fatalf("write(%q): %v", object1, err)
	}
	if err := write(client, bucket, object2); err != nil {
		t.Fatalf("write(%q): %v", object2, err)
	}

	{
		// Should only show "foo/a.txt", not "foo.txt"
		var buf bytes.Buffer
		if err := list(&buf, client, bucket); err != nil {
			t.Fatalf("cannot list objects: %v", err)
		}
		if got, want := buf.String(), object1; !strings.Contains(got, want) {
			t.Errorf("List() got %q; want to contain %q", got, want)
		}
		if got, want := buf.String(), object2; !strings.Contains(got, want) {
			t.Errorf("List() got %q; want to contain %q", got, want)
		}
	}

	{
		// Should only show "foo/a.txt", not "foo.txt"
		const prefix = "foo/"
		var buf bytes.Buffer
		if err := listByPrefix(&buf, client, bucket, prefix, ""); err != nil {
			t.Fatalf("cannot list objects by prefix: %v", err)
		}
		if got, want := buf.String(), object1; strings.Contains(got, want) {
			t.Errorf("List(%q) got %q; want NOT to contain %q", prefix, got, want)
		}
		if got, want := buf.String(), object2; !strings.Contains(got, want) {
			t.Errorf("List(%q) got %q; want to contain %q", prefix, got, want)
		}
	}

	data, err := read(client, bucket, object1)
	if err != nil {
		t.Fatalf("cannot read object: %v", err)
	}
	if got, want := string(data), "Hello\nworld"; got != want {
		t.Errorf("contents = %q; want %q", got, want)
	}
	_, err = attrs(client, bucket, object1)
	if err != nil {
		t.Errorf("cannot get object metadata: %v", err)
	}
	if err := makePublic(client, bucket, object1); err != nil {
		t.Errorf("cannot to make object public: %v", err)
	}
	err = move(client, bucket, object1)
	if err != nil {
		t.Fatalf("cannot move object: %v", err)
	}
	// object1's new name.
	object1 = object1 + "-rename"

	if err := copyToBucket(client, dstBucket, bucket, object1); err != nil {
		t.Errorf("cannot copy object to bucket: %v", err)
	}
	if err := addBucketACL(client, bucket); err != nil {
		t.Errorf("cannot add bucket acl: %v", err)
	}
	if err := addDefaultBucketACL(client, bucket); err != nil {
		t.Errorf("cannot add bucket deafult acl: %v", err)
	}
	if err := bucketACL(client, bucket); err != nil {
		t.Errorf("cannot get bucket acl: %v", err)
	}
	if err := bucketACLFiltered(client, bucket, storage.AllAuthenticatedUsers); err != nil {
		t.Errorf("cannot filter bucket acl: %v", err)
	}
	if err := deleteDefaultBucketACL(client, bucket); err != nil {
		t.Errorf("cannot delete bucket default acl: %v", err)
	}
	if err := deleteBucketACL(client, bucket); err != nil {
		t.Errorf("cannot delete bucket acl: %v", err)
	}
	if err := addObjectACL(client, bucket, object1); err != nil {
		t.Errorf("cannot add object acl: %v", err)
	}
	if err := objectACL(client, bucket, object1); err != nil {
		t.Errorf("cannot get object acl: %v", err)
	}
	if err := objectACLFiltered(client, bucket, object1, storage.AllAuthenticatedUsers); err != nil {
		t.Errorf("cannot filter object acl: %v", err)
	}
	if err := deleteObjectACL(client, bucket, object1); err != nil {
		t.Errorf("cannot delete object acl: %v", err)
	}

	key := []byte("my-secret-AES-256-encryption-key")
	newKey := []byte("My-secret-AES-256-encryption-key")
	kmsKey := ""

	if err := writeEncryptedObject(client, bucket, object1, key); err != nil {
		t.Errorf("cannot write an encrypted object: %v", err)
	}
	if err := writeWithKMSKey(client, bucket, object1, kmsKey); err != nil {
		t.Errorf("cannot write a KMS encrypted object: %v", err)
	}
	data, err = readEncryptedObject(client, bucket, object1, key)
	if err != nil {
		t.Errorf("cannot read the encrypted object: %v", err)
	}
	if got, want := string(data), "top secret"; got != want {
		t.Errorf("object content = %q; want %q", got, want)
	}
	if err := rotateEncryptionKey(client, bucket, object1, key, newKey); err != nil {
		t.Errorf("cannot encrypt the object with the new key: %v", err)
	}
	if err := delete(client, bucket, object1); err != nil {
		t.Errorf("cannot to delete object: %v", err)
	}
	if err := delete(client, bucket, object2); err != nil {
		t.Errorf("cannot to delete object: %v", err)
	}

	testutil.Retry(t, 10, time.Second, func(r *testutil.R) {
		// Cleanup, this part won't be executed if Fatal happens.
		// TODO(jbd): Implement garbage cleaning.
		if err := client.Bucket(bucket).Delete(ctx); err != nil {
			r.Errorf("cleanup of bucket failed: %v", err)
		}
	})

	testutil.Retry(t, 10, time.Second, func(r *testutil.R) {
		if err := delete(client, dstBucket, object1+"-copy"); err != nil {
			r.Errorf("cannot to delete copy object: %v", err)
		}
	})

	testutil.Retry(t, 10, time.Second, func(r *testutil.R) {
		if err := client.Bucket(dstBucket).Delete(ctx); err != nil {
			r.Errorf("cleanup of bucket failed: %v", err)
		}
	})
}

// cleanBucket ensures there's a fresh bucket with a given name, deleting the existing bucket if it already exists.
func cleanBucket(t *testing.T, ctx context.Context, client *storage.Client, projectID, bucket string) {
	b := client.Bucket(bucket)
	_, err := b.Attrs(ctx)
	if err == nil {
		it := b.Objects(ctx, nil)
		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatalf("Bucket.Objects(%q): %v", bucket, err)
			}
			if err := b.Object(attrs.Name).Delete(ctx); err != nil {
				t.Fatalf("Bucket(%q).Object(%q).Delete: %v", bucket, attrs.Name, err)
			}
		}
		if err := b.Delete(ctx); err != nil {
			t.Fatalf("Bucket.Delete(%q): %v", bucket, err)
		}
	}
	if err := b.Create(ctx, projectID, nil); err != nil {
		t.Fatalf("Bucket.Create(%q): %v", bucket, err)
	}
}
