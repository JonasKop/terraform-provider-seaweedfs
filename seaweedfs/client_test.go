package seaweedfs

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIAMClientUserLifecycle(t *testing.T) {
	t.Parallel()

	users := map[string]bool{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		_ = r.Body.Close()

		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}

		action := form.Get("Action")
		name := form.Get("UserName")

		switch action {
		case "CreateUser":
			users[name] = true
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<CreateUserResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><CreateUserResult><User><Path>/</Path><UserName>` + name + `</UserName><UserId>uid-123</UserId><Arn>arn:aws:iam::123456789012:user/` + name + `</Arn></User></CreateUserResult></CreateUserResponse>`))
		case "GetUser":
			if !users[name] {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Code>NoSuchEntity</Code><Message>Not found</Message></Error></ErrorResponse>`))
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<GetUserResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><GetUserResult><User><Path>/</Path><UserName>` + name + `</UserName><UserId>uid-123</UserId><Arn>arn:aws:iam::123456789012:user/` + name + `</Arn></User></GetUserResult></GetUserResponse>`))
		case "DeleteUser":
			delete(users, name)
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<DeleteUserResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></DeleteUserResponse>`))
		default:
			t.Fatalf("unexpected action: %s", action)
		}
	}))
	defer srv.Close()

	client, err := newIAMClient(iamClientConfig{
		Endpoint:  srv.URL,
		Region:    "us-east-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	userName := "test-user"

	created, err := client.CreateUser(ctx, userName, "/")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.User.UserName != userName {
		t.Fatalf("expected created username %q, got %q", userName, created.User.UserName)
	}

	readUser, err := client.GetUser(ctx, userName)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if readUser.User.UserName != userName {
		t.Fatalf("expected read username %q, got %q", userName, readUser.User.UserName)
	}

	if err := client.DeleteUser(ctx, userName); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err = client.GetUser(ctx, userName)
	if err == nil {
		t.Fatal("expected no-such-entity error after delete, got nil")
	}
	if !isNoSuchEntityError(err) {
		t.Fatalf("expected NoSuchEntity error, got: %v", err)
	}
}

func TestIAMClientCreateFromServiceFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Code>ServiceFailure</Code><Message>Internal server error</Message></Error></ErrorResponse>`))
	}))
	defer srv.Close()

	client, err := newIAMClient(iamClientConfig{
		Endpoint:  srv.URL,
		Region:    "us-east-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.CreateUser(context.Background(), "x", "/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ServiceFailure") {
		t.Fatalf("expected ServiceFailure in error, got: %v", err)
	}
}

func TestIAMClientAccessKeyPolicyAndBucket(t *testing.T) {
	t.Parallel()

	users := map[string]bool{"alice": true}
	keys := map[string]string{}
	policies := map[string]string{}
	buckets := map[string]bool{}
	bucketTags := map[string]map[string]string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasTagging := r.URL.Query()["tagging"]
		if hasTagging {
			bucket := strings.TrimPrefix(r.URL.Path, "/")
			if !buckets[bucket] {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code><Message>Not Found</Message></Error>`))
				return
			}

			switch r.Method {
			case http.MethodGet:
				tags := bucketTags[bucket]
				if len(tags) == 0 {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`<Error><Code>NoSuchTagSet</Code><Message>No tags</Message></Error>`))
					return
				}
				var out s3Tagging
				for key, value := range tags {
					out.TagSet = append(out.TagSet, s3Tag{
						Key:   key,
						Value: value,
					})
				}
				data, err := xml.Marshal(out)
				if err != nil {
					t.Fatalf("marshal tagging xml: %v", err)
				}
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write(data)
			case http.MethodPut:
				var in s3Tagging
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read tagging body: %v", err)
				}
				if err := xml.Unmarshal(body, &in); err != nil {
					t.Fatalf("unmarshal tagging body: %v", err)
				}
				tags := map[string]string{}
				for _, tag := range in.TagSet {
					tags[tag.Key] = tag.Value
				}
				bucketTags[bucket] = tags
				w.WriteHeader(http.StatusOK)
			case http.MethodDelete:
				delete(bucketTags, bucket)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Fatalf("unexpected tagging method: %s", r.Method)
			}
			return
		}

		if r.Method == http.MethodPut || r.Method == http.MethodHead || r.Method == http.MethodDelete {
			bucket := strings.TrimPrefix(r.URL.Path, "/")
			switch r.Method {
			case http.MethodPut:
				buckets[bucket] = true
				w.WriteHeader(http.StatusOK)
			case http.MethodHead:
				if !buckets[bucket] {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code><Message>Not Found</Message></Error>`))
					return
				}
				w.WriteHeader(http.StatusOK)
			case http.MethodDelete:
				delete(buckets, bucket)
				w.WriteHeader(http.StatusNoContent)
			}
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		_ = r.Body.Close()

		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}

		action := form.Get("Action")
		user := form.Get("UserName")

		switch action {
		case "CreateAccessKey":
			if !users[user] {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<ErrorResponse><Error><Code>NoSuchEntity</Code><Message>Not found</Message></Error></ErrorResponse>`))
				return
			}
			keys[user] = "AKIA_TEST"
			_, _ = w.Write([]byte(`<CreateAccessKeyResponse><CreateAccessKeyResult><AccessKey><UserName>` + user + `</UserName><AccessKeyId>AKIA_TEST</AccessKeyId><Status>Active</Status><SecretAccessKey>SECRET123</SecretAccessKey></AccessKey></CreateAccessKeyResult></CreateAccessKeyResponse>`))
		case "ListAccessKeys":
			if keys[user] == "" {
				_, _ = w.Write([]byte(`<ListAccessKeysResponse><ListAccessKeysResult><AccessKeyMetadata></AccessKeyMetadata></ListAccessKeysResult></ListAccessKeysResponse>`))
				return
			}
			_, _ = w.Write([]byte(`<ListAccessKeysResponse><ListAccessKeysResult><AccessKeyMetadata><member><UserName>` + user + `</UserName><AccessKeyId>` + keys[user] + `</AccessKeyId><Status>Active</Status></member></AccessKeyMetadata></ListAccessKeysResult></ListAccessKeysResponse>`))
		case "DeleteAccessKey":
			delete(keys, user)
			_, _ = w.Write([]byte(`<DeleteAccessKeyResponse/>`))
		case "PutUserPolicy":
			policies[user+":"+form.Get("PolicyName")] = form.Get("PolicyDocument")
			_, _ = w.Write([]byte(`<PutUserPolicyResponse/>`))
		case "GetUserPolicy":
			key := user + ":" + form.Get("PolicyName")
			val, ok := policies[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<ErrorResponse><Error><Code>NoSuchEntity</Code><Message>Not found</Message></Error></ErrorResponse>`))
				return
			}
			_, _ = w.Write([]byte(`<GetUserPolicyResponse><GetUserPolicyResult><UserName>` + user + `</UserName><PolicyName>` + form.Get("PolicyName") + `</PolicyName><PolicyDocument>` + val + `</PolicyDocument></GetUserPolicyResult></GetUserPolicyResponse>`))
		case "DeleteUserPolicy":
			delete(policies, user+":"+form.Get("PolicyName"))
			_, _ = w.Write([]byte(`<DeleteUserPolicyResponse/>`))
		default:
			t.Fatalf("unexpected action: %s", action)
		}
	}))
	defer srv.Close()

	client, err := newIAMClient(iamClientConfig{
		Endpoint:  srv.URL,
		Region:    "us-east-1",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()

	ak, err := client.CreateAccessKey(ctx, "alice")
	if err != nil {
		t.Fatalf("create access key: %v", err)
	}
	if ak.AccessKeyID != "AKIA_TEST" || ak.SecretAccessKey != "SECRET123" {
		t.Fatalf("unexpected access key response: %+v", ak)
	}

	list, err := client.ListAccessKeys(ctx, "alice")
	if err != nil {
		t.Fatalf("list access keys: %v", err)
	}
	if len(list) != 1 || list[0].AccessKeyID != "AKIA_TEST" {
		t.Fatalf("unexpected key list: %+v", list)
	}

	if err := client.PutUserPolicy(ctx, "alice", "p1", "%7B%22Version%22%3A%222012-10-17%22%7D"); err != nil {
		t.Fatalf("put user policy: %v", err)
	}
	pol, err := client.GetUserPolicy(ctx, "alice", "p1")
	if err != nil {
		t.Fatalf("get user policy: %v", err)
	}
	if !strings.Contains(pol, "Version") {
		t.Fatalf("unexpected policy decode: %s", pol)
	}
	if err := client.DeleteUserPolicy(ctx, "alice", "p1"); err != nil {
		t.Fatalf("delete user policy: %v", err)
	}

	if err := client.CreateBucket(ctx, "b1"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	if err := client.HeadBucket(ctx, "b1"); err != nil {
		t.Fatalf("head bucket: %v", err)
	}

	tags, err := client.GetBucketTags(ctx, "b1")
	if err != nil {
		t.Fatalf("get empty bucket tags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected no tags, got: %+v", tags)
	}

	if err := client.PutBucketTags(ctx, "b1", map[string]string{
		"env":  "prod",
		"team": "platform",
	}); err != nil {
		t.Fatalf("put bucket tags: %v", err)
	}
	tags, err = client.GetBucketTags(ctx, "b1")
	if err != nil {
		t.Fatalf("get bucket tags: %v", err)
	}
	if tags["env"] != "prod" || tags["team"] != "platform" {
		t.Fatalf("unexpected tags: %+v", tags)
	}

	if err := client.DeleteBucketTags(ctx, "b1"); err != nil {
		t.Fatalf("delete bucket tags: %v", err)
	}
	tags, err = client.GetBucketTags(ctx, "b1")
	if err != nil {
		t.Fatalf("get empty bucket tags after delete: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected no tags after delete, got: %+v", tags)
	}

	if err := client.DeleteBucket(ctx, "b1"); err != nil {
		t.Fatalf("delete bucket: %v", err)
	}

	if err := client.DeleteAccessKey(ctx, "alice", "AKIA_TEST"); err != nil {
		t.Fatalf("delete access key: %v", err)
	}
}

func TestPoliciesSemanticallyEqual(t *testing.T) {
	t.Parallel()

	a := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:*"],"Resource":"*"}]}`
	b := `{
  "Statement": [
    {
      "Resource": "*",
      "Action": [
        "s3:*"
      ],
      "Effect": "Allow"
    }
  ],
  "Version": "2012-10-17"
}`

	if !policiesSemanticallyEqual(a, b) {
		t.Fatalf("expected policies to be semantically equal")
	}
}

func TestRetryIAMEventuallyConsistent(t *testing.T) {
	t.Parallel()

	attempts := 0
	err := retryIAMEventuallyConsistent(context.Background(), 4, func() error {
		attempts++
		if attempts < 3 {
			return iamError{Code: "ServiceFailure", Message: "temporary"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryIAMEventuallyConsistentStopsOnNonRetryableError(t *testing.T) {
	t.Parallel()

	nonRetryable := errors.New("boom")
	attempts := 0
	err := retryIAMEventuallyConsistent(context.Background(), 5, func() error {
		attempts++
		return nonRetryable
	})
	if !errors.Is(err, nonRetryable) {
		t.Fatalf("expected non-retryable error, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestIAMErrorHelpers(t *testing.T) {
	t.Parallel()

	if !isEntityAlreadyExistsError(iamError{Code: "EntityAlreadyExists", Message: "exists"}) {
		t.Fatalf("expected EntityAlreadyExists helper to match")
	}
	if !isBucketAlreadyExistsError(iamError{Code: "BucketAlreadyOwnedByYou", Message: "exists"}) {
		t.Fatalf("expected BucketAlreadyOwnedByYou helper to match")
	}
	if !isRetryableIAMError(iamError{Code: "ServiceFailure", Message: "temporary"}) {
		t.Fatalf("expected ServiceFailure to be retryable")
	}
}
