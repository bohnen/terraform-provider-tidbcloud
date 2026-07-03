package provider

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	mockClient "github.com/tidbcloud/terraform-provider-tidbcloud/mock"
	"github.com/tidbcloud/terraform-provider-tidbcloud/tidbcloud"
	impV1beta1 "github.com/tidbcloud/tidbcloud-cli/pkg/tidbcloud/v1beta1/serverless/imp"
)

func TestAccServerlessImportResource(t *testing.T) {
	serverlessImportResourceName := "tidbcloud_serverless_import.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccServerlessImportResourceConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(serverlessImportResourceName, "import_options.file_type", "CSV"),
					resource.TestCheckResourceAttr(serverlessImportResourceName, "source.type", "S3"),
					resource.TestCheckResourceAttrSet(serverlessImportResourceName, "import_id"),
					resource.TestCheckResourceAttrSet(serverlessImportResourceName, "state"),
				),
			},
		},
	})
}

func TestUTServerlessImportResource(t *testing.T) {
	setupTestEnv()

	ctrl := gomock.NewController(t)
	s := mockClient.NewMockTiDBCloudServerlessClient(ctrl)
	defer HookGlobal(&NewServerlessClient, func(publicKey string, privateKey string, serverlessEndpoint string, userAgent string) (tidbcloud.TiDBCloudServerlessClient, error) {
		return s, nil
	})()

	importId := "import-id"

	createImportResp := impV1beta1.Import{}
	createImportResp.UnmarshalJSON([]byte(testUTImport(string(impV1beta1.IMPORTSTATEENUM_IMPORTING))))
	getImportResp := impV1beta1.Import{}
	getImportResp.UnmarshalJSON([]byte(testUTImport(string(impV1beta1.IMPORTSTATEENUM_COMPLETED))))

	s.EXPECT().CreateImport(gomock.Any(), gomock.Any(), gomock.Any()).Return(&createImportResp, nil)
	s.EXPECT().GetImport(gomock.Any(), gomock.Any(), importId).Return(&getImportResp, nil).AnyTimes()

	testServerlessImportResource(t)
}

func TestUTServerlessImportResourceCancelOnDelete(t *testing.T) {
	setupTestEnv()

	ctrl := gomock.NewController(t)
	s := mockClient.NewMockTiDBCloudServerlessClient(ctrl)
	defer HookGlobal(&NewServerlessClient, func(publicKey string, privateKey string, serverlessEndpoint string, userAgent string) (tidbcloud.TiDBCloudServerlessClient, error) {
		return s, nil
	})()

	importId := "import-id"

	importingResp := impV1beta1.Import{}
	importingResp.UnmarshalJSON([]byte(testUTImport(string(impV1beta1.IMPORTSTATEENUM_IMPORTING))))

	s.EXPECT().CreateImport(gomock.Any(), gomock.Any(), gomock.Any()).Return(&importingResp, nil)
	s.EXPECT().GetImport(gomock.Any(), gomock.Any(), importId).Return(&importingResp, nil).AnyTimes()
	// the import is still running when the resource is destroyed, so it must be canceled
	s.EXPECT().CancelImport(gomock.Any(), gomock.Any(), importId).Return(nil)

	testServerlessImportResource(t)
}

func testServerlessImportResource(t *testing.T) {
	serverlessImportResourceName := "tidbcloud_serverless_import.test"
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read serverless import resource
			{
				Config: testUTServerlessImportResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(serverlessImportResourceName, "import_id", "import-id"),
					resource.TestCheckResourceAttr(serverlessImportResourceName, "import_options.file_type", "CSV"),
					resource.TestCheckResourceAttr(serverlessImportResourceName, "source.type", "S3"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccServerlessImportResourceConfig() string {
	return `
resource "tidbcloud_serverless_cluster" "example" {
   display_name = "test-tf"
   region = {
      name = "regions/aws-us-east-1"
   }
}
resource "tidbcloud_serverless_import" "test" {
	cluster_id = tidbcloud_serverless_cluster.example.cluster_id
	import_options = {
		file_type = "CSV"
	}
	source = {
		type = "S3"
		s3 = {
			uri       = "s3://test-bucket/test-path/"
			auth_type = "ROLE_ARN"
			role_arn  = "arn:aws:iam::123456789012:role/test-role"
		}
	}
}
`
}

func testUTServerlessImportResourceConfig() string {
	return `
resource "tidbcloud_serverless_import" "test" {
	cluster_id = "cluster_id"
	import_options = {
		file_type = "CSV"
	}
	source = {
		type = "S3"
		s3 = {
			uri       = "s3://test-bucket/test-path/"
			auth_type = "ROLE_ARN"
			role_arn  = "arn:aws:iam::123456789012:role/test-role"
		}
	}
}
`
}

func testUTImport(state string) string {
	return fmt.Sprintf(`
{
    "importId": "import-id",
    "name": "clusters/cluster-id/imports/import-id",
    "clusterId": "cluster-id",
    "totalSize": "100",
    "createTime": "2025-03-20T05:53:57.000Z",
    "state": "%s",
    "completePercent": 100,
    "message": "",
    "createdBy": "apikey-S22Jxxxxx",
    "creationDetails": {
        "importOptions": {
            "fileType": "CSV"
        },
        "source": {
            "type": "S3",
            "s3": {
                "uri": "s3://test-bucket/test-path/",
                "authType": "ROLE_ARN",
                "roleArn": "arn:aws:iam::123456789012:role/test-role"
            }
        }
    }
}
`, state)
}
