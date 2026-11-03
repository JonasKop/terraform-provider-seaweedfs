package seaweedfs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIAMUserPolicyReadDetectsDrift(t *testing.T) {
	const original = `{"Statement":[{"Action":"s3:GetObject","Effect":"Allow"}]}`
	const changed = `{"Statement":[{"Action":"s3:PutObject","Effect":"Allow"}]}`
	for _, tc := range []struct {
		name, remote, want string
	}{
		{"unchanged formatting", `{ "Statement": [ { "Effect": "Allow", "Action": "s3:GetObject" } ] }`, original},
		{"unchanged singleton arrays", `{"Statement":[{"Action":["s3:GetObject"],"Effect":"Allow"}]}`, original},
		{"unchanged singleton statement", `{"Statement":{"Action":"s3:GetObject","Effect":"Allow"}}`, original},
		{"changed policy", changed, changed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil || r.Form.Get("Action") != "GetUserPolicy" {
					t.Errorf("unexpected IAM request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				fmt.Fprintf(w, `<GetUserPolicyResponse><GetUserPolicyResult><PolicyDocument>%s</PolicyDocument></GetUserPolicyResult></GetUserPolicyResponse>`, url.QueryEscape(tc.remote))
			}))
			defer server.Close()

			client, err := newIAMClient(iamClientConfig{Endpoint: server.URL, Region: "us-east-1", AccessKey: "test-key", SecretKey: "test-secret"})
			if err != nil {
				t.Fatal(err)
			}
			r := &iamUserPolicyResource{client: client}
			var schemaResp resource.SchemaResponse
			r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
			state := tfsdk.State{Schema: schemaResp.Schema}
			model := iamUserPolicyResourceModel{
				ID: types.StringValue("test-user:test-policy"), UserName: types.StringValue("test-user"),
				Name: types.StringValue("test-policy"), Policy: types.StringValue(original),
			}
			if diags := state.Set(context.Background(), &model); diags.HasError() {
				t.Fatalf("set state: %v", diags)
			}
			resp := resource.ReadResponse{State: state}
			r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("read: %v", resp.Diagnostics)
			}
			if diags := resp.State.Get(context.Background(), &model); diags.HasError() {
				t.Fatalf("get state: %v", diags)
			}
			if got := model.Policy.ValueString(); got != tc.want {
				t.Fatalf("policy = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPolicyComparisonPreservesMeaningfulDifferences(t *testing.T) {
	for _, key := range []string{"Action", "NotAction", "Resource", "NotResource"} {
		scalar := fmt.Sprintf(`{"Statement":[{"%s":"value"}]}`, key)
		list := fmt.Sprintf(`{"Statement":[{"%s":["value"]}]}`, key)
		if !policiesSemanticallyEqual(scalar, list) {
			t.Errorf("%s: singleton list should match its scalar", key)
		}
	}
	if policiesSemanticallyEqual(`{"limit":9007199254740992}`, `{"limit":9007199254740993}`) {
		t.Error("different large JSON integers must not compare equal")
	}
}
