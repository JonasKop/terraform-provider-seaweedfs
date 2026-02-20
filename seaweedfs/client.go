package seaweedfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type iamClientConfig struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Insecure  bool
}

type iamClient struct {
	endpoint string
	region   string
	creds    aws.CredentialsProvider
	signer   *v4.Signer
	http     *http.Client
	s3       *s3.Client
}

type iamError struct {
	Code    string
	Message string
}

type iamErrorEnvelope struct {
	Code    string      `xml:"Code"`
	Message string      `xml:"Message"`
	Error   iamAPIError `xml:"Error"`
}

type iamAPIError struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

type createAccessKeyResponse struct {
	AccessKey iamAccessKey `xml:"CreateAccessKeyResult>AccessKey"`
}

type listAccessKeysResponse struct {
	Items []iamAccessKeyMetadata `xml:"ListAccessKeysResult>AccessKeyMetadata>member"`
}

type getUserPolicyResponse struct {
	UserName       string `xml:"GetUserPolicyResult>UserName"`
	PolicyName     string `xml:"GetUserPolicyResult>PolicyName"`
	PolicyDocument string `xml:"GetUserPolicyResult>PolicyDocument"`
}

type iamAccessKey struct {
	UserName        string `xml:"UserName"`
	AccessKeyID     string `xml:"AccessKeyId"`
	Status          string `xml:"Status"`
	SecretAccessKey string `xml:"SecretAccessKey"`
}

type iamAccessKeyMetadata struct {
	UserName    string `xml:"UserName"`
	AccessKeyID string `xml:"AccessKeyId"`
	Status      string `xml:"Status"`
}

type s3Tagging struct {
	XMLName xml.Name `xml:"Tagging"`
	TagSet  []s3Tag  `xml:"TagSet>Tag"`
}

type s3Tag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

func (e iamError) Error() string {
	if e.Code == "" && e.Message == "" {
		return "unknown IAM error"
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type createUserResponse struct {
	User iamUser `xml:"CreateUserResult>User"`
}

type getUserResponse struct {
	User iamUser `xml:"GetUserResult>User"`
}

type iamUser struct {
	UserName string `xml:"UserName"`
	Arn      string `xml:"Arn"`
	UserID   string `xml:"UserId"`
	Path     string `xml:"Path"`
}

func newIAMClient(cfg iamClientConfig) (*iamClient, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("access_key and secret_key are required")
	}

	tr := &http.Transport{}
	if cfg.Insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	client := &iamClient{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		region:   cfg.Region,
		creds: credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		),
		signer: v4.NewSigner(func(o *v4.SignerOptions) {
			o.DisableURIPathEscaping = true
		}),
		http: &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second,
		},
	}

	client.s3 = s3.New(s3.Options{
		Region:       client.region,
		Credentials:  client.creds,
		HTTPClient:   client.http,
		BaseEndpoint: aws.String(client.endpoint),
		UsePathStyle: true,
	})

	return client, nil
}

func (c *iamClient) CreateUser(ctx context.Context, userName string, path string) (getUserResponse, error) {
	vals := url.Values{}
	vals.Set("Action", "CreateUser")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)
	if path != "" {
		vals.Set("Path", path)
	}

	var out createUserResponse
	if err := c.doIAMAction(ctx, vals, &out); err != nil {
		return getUserResponse{}, err
	}

	return getUserResponse{User: out.User}, nil
}

func (c *iamClient) GetUser(ctx context.Context, userName string) (getUserResponse, error) {
	vals := url.Values{}
	vals.Set("Action", "GetUser")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)

	var out getUserResponse
	if err := c.doIAMAction(ctx, vals, &out); err != nil {
		return getUserResponse{}, err
	}
	return out, nil
}

func (c *iamClient) DeleteUser(ctx context.Context, userName string) error {
	vals := url.Values{}
	vals.Set("Action", "DeleteUser")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)

	return c.doIAMAction(ctx, vals, nil)
}

func (c *iamClient) CreateAccessKey(ctx context.Context, userName string) (iamAccessKey, error) {
	vals := url.Values{}
	vals.Set("Action", "CreateAccessKey")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)

	var out createAccessKeyResponse
	if err := c.doIAMAction(ctx, vals, &out); err != nil {
		return iamAccessKey{}, err
	}

	return out.AccessKey, nil
}

func (c *iamClient) ListAccessKeys(ctx context.Context, userName string) ([]iamAccessKeyMetadata, error) {
	vals := url.Values{}
	vals.Set("Action", "ListAccessKeys")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)

	var out listAccessKeysResponse
	if err := c.doIAMAction(ctx, vals, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *iamClient) DeleteAccessKey(ctx context.Context, userName string, accessKeyID string) error {
	vals := url.Values{}
	vals.Set("Action", "DeleteAccessKey")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)
	vals.Set("AccessKeyId", accessKeyID)

	return c.doIAMAction(ctx, vals, nil)
}

func (c *iamClient) PutUserPolicy(ctx context.Context, userName string, policyName string, policyDocument string) error {
	vals := url.Values{}
	vals.Set("Action", "PutUserPolicy")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)
	vals.Set("PolicyName", policyName)
	vals.Set("PolicyDocument", policyDocument)

	return c.doIAMAction(ctx, vals, nil)
}

func (c *iamClient) GetUserPolicy(ctx context.Context, userName string, policyName string) (string, error) {
	vals := url.Values{}
	vals.Set("Action", "GetUserPolicy")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)
	vals.Set("PolicyName", policyName)

	var out getUserPolicyResponse
	if err := c.doIAMAction(ctx, vals, &out); err != nil {
		return "", err
	}

	decoded, err := url.QueryUnescape(out.PolicyDocument)
	if err != nil {
		return out.PolicyDocument, nil
	}
	return decoded, nil
}

func (c *iamClient) DeleteUserPolicy(ctx context.Context, userName string, policyName string) error {
	vals := url.Values{}
	vals.Set("Action", "DeleteUserPolicy")
	vals.Set("Version", "2010-05-08")
	vals.Set("UserName", userName)
	vals.Set("PolicyName", policyName)

	return c.doIAMAction(ctx, vals, nil)
}

func (c *iamClient) CreateBucket(ctx context.Context, name string) error {
	path := "/" + name
	_, err := c.doSignedRequest(ctx, "s3", http.MethodPut, c.endpoint+path, "", "", nil)
	return err
}

func (c *iamClient) HeadBucket(ctx context.Context, name string) error {
	path := "/" + name
	_, err := c.doSignedRequest(ctx, "s3", http.MethodHead, c.endpoint+path, "", "", nil)
	return err
}

func (c *iamClient) DeleteBucket(ctx context.Context, name string) error {
	path := "/" + name
	_, err := c.doSignedRequest(ctx, "s3", http.MethodDelete, c.endpoint+path, "", "", nil)
	return err
}

func (c *iamClient) GetBucketTags(ctx context.Context, name string) (map[string]string, error) {
	out, err := c.s3.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		if isNoSuchTagSetError(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("get bucket tagging: %w", err)
	}

	tags := make(map[string]string, len(out.TagSet))
	for _, tag := range out.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags, nil
}

func (c *iamClient) PutBucketTags(ctx context.Context, name string, tags map[string]string) error {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tagSet := make([]s3types.Tag, 0, len(tags))
	for _, key := range keys {
		value := tags[key]
		tagSet = append(tagSet, s3types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	_, err := c.s3.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(name),
		Tagging: &s3types.Tagging{
			TagSet: tagSet,
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket tagging: %w", err)
	}
	return nil
}

func (c *iamClient) DeleteBucketTags(ctx context.Context, name string) error {
	_, err := c.s3.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("delete bucket tagging: %w", err)
	}
	return nil
}

func (c *iamClient) doIAMAction(ctx context.Context, form url.Values, out any) error {
	body := form.Encode()
	_, err := c.doSignedRequest(
		ctx,
		"iam",
		http.MethodPost,
		c.endpoint+"/",
		"application/x-www-form-urlencoded",
		body,
		out,
	)
	return err
}

func (c *iamClient) doSignedRequest(
	ctx context.Context,
	service string,
	method string,
	requestURL string,
	contentType string,
	body string,
	out any,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Host", req.URL.Host)

	creds, err := c.creds.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieve credentials: %w", err)
	}

	sum := sha256.Sum256([]byte(body))
	hash := fmt.Sprintf("%x", sum)
	ctx = v4.SetPayloadHash(ctx, hash)

	if err := c.signer.SignHTTP(ctx, creds, req, hash, service, c.region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp.StatusCode, data)
	}

	if out != nil {
		if err := xml.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("decode xml response: %w", err)
		}
	}

	return data, nil
}

func parseAPIError(status int, data []byte) error {
	var envelope iamErrorEnvelope
	if xmlErr := xml.Unmarshal(data, &envelope); xmlErr == nil {
		apiErr := iamError{
			Code:    envelope.Error.Code,
			Message: envelope.Error.Message,
		}
		if apiErr.Code == "" {
			apiErr.Code = envelope.Code
		}
		if apiErr.Message == "" {
			apiErr.Message = envelope.Message
		}
		if apiErr.Code == "" || apiErr.Message == "" {
			var direct iamAPIError
			if xmlErr := xml.Unmarshal(data, &direct); xmlErr == nil {
				if apiErr.Code == "" {
					apiErr.Code = direct.Code
				}
				if apiErr.Message == "" {
					apiErr.Message = direct.Message
				}
			}
		}
		if apiErr.Code != "" || apiErr.Message != "" {
			if apiErr.Code == "" {
				apiErr.Code = fmt.Sprintf("HTTP%d", status)
			}
			if apiErr.Message == "" {
				apiErr.Message = strings.TrimSpace(string(data))
			}
			return apiErr
		}
	}

	return iamError{
		Code:    fmt.Sprintf("HTTP%d", status),
		Message: strings.TrimSpace(string(data)),
	}
}

func isNoSuchEntityError(err error) bool {
	var apiErr iamError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "NoSuchEntity"
	}
	return false
}

func isEntityAlreadyExistsError(err error) bool {
	var apiErr iamError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "EntityAlreadyExists"
	}
	return false
}

func isServiceFailureError(err error) bool {
	var apiErr iamError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "ServiceFailure" || apiErr.Code == "HTTP500" || apiErr.Code == "HTTP503"
	}
	return false
}

func isRetryableIAMError(err error) bool {
	return isNoSuchEntityError(err) || isServiceFailureError(err)
}

func retryIAMEventuallyConsistent(ctx context.Context, attempts int, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}

	delay := 200 * time.Millisecond
	var lastErr error

	for i := 0; i < attempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableIAMError(err) {
			return err
		}

		lastErr = err
		if i == attempts-1 {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		delay *= 2
		if delay > 2*time.Second {
			delay = 2 * time.Second
		}
	}

	return lastErr
}

func isNoSuchBucketError(err error) bool {
	var apiErr iamError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "NoSuchBucket" || apiErr.Code == "NotFound" || apiErr.Code == "NoSuchKey" || apiErr.Code == "NoSuchEntity"
	}
	return false
}

func isBucketAlreadyExistsError(err error) bool {
	var apiErr iamError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "BucketAlreadyOwnedByYou" || apiErr.Code == "BucketAlreadyExists"
	}
	return false
}

func isNoSuchTagSetError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NoSuchTagSet"
	}
	return false
}
