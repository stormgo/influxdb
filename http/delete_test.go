package http

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/influxdata/influxdb"
	pcontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/mock"
	influxtesting "github.com/influxdata/influxdb/testing"
	"go.uber.org/zap"
)

// NewMockDeleteBackend returns a DeleteBackend with mock services.
func NewMockDeleteBackend() *DeleteBackend {
	return &DeleteBackend{
		Logger: zap.NewNop().With(zap.String("handler", "delete")),

		DeleteService:       mock.NewDeleteService(),
		BucketService:       mock.NewBucketService(),
		OrganizationService: mock.NewOrganizationService(),
	}
}

func TestDelete(t *testing.T) {
	type fields struct {
		DeleteService       influxdb.DeleteService
		OrganizationService influxdb.OrganizationService
		BucketService       influxdb.BucketService
	}

	type args struct {
		queryParams map[string][]string
		body        []byte
		authorizer  influxdb.Authorizer
	}

	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "missing org",
			args: args{
				queryParams: map[string][]string{},
				body:        []byte("{}"),
				authorizer:  &influxdb.Authorization{UserID: user1ID},
			},
			fields: fields{
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return nil, &influxdb.Error{
							Code: influxdb.EInvalid,
							Msg:  "Please provide either orgID or org",
						}
					},
				},
			},
			wants: wants{
				statusCode:  http.StatusBadRequest,
				contentType: "application/json; charset=utf-8",
				body: fmt.Sprintf(`{
					"code": "invalid",
					"message": "Please provide either orgID or org"
				  }`),
			},
		},
		{
			name: "missing bucket",
			args: args{
				queryParams: map[string][]string{
					"org": []string{"org1"},
				},
				body:       []byte("{}"),
				authorizer: &influxdb.Authorization{UserID: user1ID},
			},
			fields: fields{
				BucketService: &mock.BucketService{
					FindBucketFn: func(ctx context.Context, f influxdb.BucketFilter) (*influxdb.Bucket, error) {
						return nil, &influxdb.Error{
							Code: influxdb.EInvalid,
							Msg:  "Please provide either bucketID or bucket",
						}
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return &influxdb.Organization{
							ID: influxdb.ID(1),
						}, nil
					},
				},
			},
			wants: wants{
				statusCode:  http.StatusBadRequest,
				contentType: "application/json; charset=utf-8",
				body: fmt.Sprintf(`{
					"code": "invalid",
					"message": "Please provide either bucketID or bucket"
				  }`),
			},
		},
		{
			name: "insufficient permissions delete",
			args: args{
				queryParams: map[string][]string{
					"org":    []string{"org1"},
					"bucket": []string{"buck1"},
				},
				body:       []byte("{}"),
				authorizer: &influxdb.Authorization{UserID: user1ID},
			},
			fields: fields{
				BucketService: &mock.BucketService{
					FindBucketFn: func(ctx context.Context, f influxdb.BucketFilter) (*influxdb.Bucket, error) {
						return &influxdb.Bucket{
							ID:   influxdb.ID(2),
							Name: "bucket1",
						}, nil
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return &influxdb.Organization{
							ID: influxdb.ID(1),
						}, nil
					},
				},
			},
			wants: wants{
				statusCode:  http.StatusForbidden,
				contentType: "application/json; charset=utf-8",
				body: fmt.Sprintf(`{
					"code": "forbidden",
					"message": "insufficient permissions for write"
				  }`),
			},
		},
		{
			name: "blank delete",
			args: args{
				queryParams: map[string][]string{
					"org":    []string{"org1"},
					"bucket": []string{"buck1"},
				},
				body: []byte("{}"),
				authorizer: &influxdb.Authorization{
					UserID: user1ID,
					Status: influxdb.Active,
					Permissions: []influxdb.Permission{
						{
							Action: influxdb.WriteAction,
							Resource: influxdb.Resource{
								Type:  influxdb.BucketsResourceType,
								ID:    influxtesting.IDPtr(influxdb.ID(2)),
								OrgID: influxtesting.IDPtr(influxdb.ID(1)),
							},
						},
					},
				},
			},
			fields: fields{
				DeleteService: mock.NewDeleteService(),
				BucketService: &mock.BucketService{
					FindBucketFn: func(ctx context.Context, f influxdb.BucketFilter) (*influxdb.Bucket, error) {
						return &influxdb.Bucket{
							ID:   influxdb.ID(2),
							Name: "bucket1",
						}, nil
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return &influxdb.Organization{
							ID:   influxdb.ID(1),
							Name: "org1",
						}, nil
					},
				},
			},
			wants: wants{
				statusCode: http.StatusNoContent,
				body:       fmt.Sprintf(``),
			},
		},
		{
			name: "complex delete",
			args: args{
				queryParams: map[string][]string{
					"org":    []string{"org1"},
					"bucket": []string{"buck1"},
				},
				body: []byte(`{
					"nodeType": "logical",
					"operator":"and",
					"children":[
						{
							"nodeType":"tagRule",
							"operator":"equal",
							"key":"tag1",
							"value":"v1"
						},
						{
							"nodeType":"logical",
							"operator":"or",
							"children":[
								{
									"nodeType":"tagRule",
									"operator":"notequal",
									"key":"tag2",
									"value":"v2"
								},
								{
									"nodeType":"tagRule",
									"operator":"regexequal",
									"key":"tag3",
									"value":"/v3/"
								}
							]							
						}
					]
				}`),
				authorizer: &influxdb.Authorization{
					UserID: user1ID,
					Status: influxdb.Active,
					Permissions: []influxdb.Permission{
						{
							Action: influxdb.WriteAction,
							Resource: influxdb.Resource{
								Type:  influxdb.BucketsResourceType,
								ID:    influxtesting.IDPtr(influxdb.ID(2)),
								OrgID: influxtesting.IDPtr(influxdb.ID(1)),
							},
						},
					},
				},
			},
			fields: fields{
				DeleteService: mock.NewDeleteService(),
				BucketService: &mock.BucketService{
					FindBucketFn: func(ctx context.Context, f influxdb.BucketFilter) (*influxdb.Bucket, error) {
						return &influxdb.Bucket{
							ID:   influxdb.ID(2),
							Name: "bucket1",
						}, nil
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return &influxdb.Organization{
							ID:   influxdb.ID(1),
							Name: "org1",
						}, nil
					},
				},
			},
			wants: wants{
				statusCode: http.StatusNoContent,
				body:       fmt.Sprintf(``),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteBackend := NewMockDeleteBackend()
			deleteBackend.HTTPErrorHandler = ErrorHandler(0)
			deleteBackend.DeleteService = tt.fields.DeleteService
			deleteBackend.OrganizationService = tt.fields.OrganizationService
			deleteBackend.BucketService = tt.fields.BucketService
			h := NewDeleteHandler(deleteBackend)

			r := httptest.NewRequest("POST", "http://any.tld", bytes.NewReader(tt.args.body))

			qp := r.URL.Query()
			for k, vs := range tt.args.queryParams {
				for _, v := range vs {
					qp.Add(k, v)
				}
			}
			r = r.WithContext(pcontext.SetAuthorizer(r.Context(), tt.args.authorizer))
			r.URL.RawQuery = qp.Encode()

			w := httptest.NewRecorder()

			h.handleDelete(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleDelete() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleDelete() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handleDelete(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handleDelete() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}